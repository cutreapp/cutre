package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/database"
	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/handler/account"
	"github.com/cutreapp/cutre/go/internal/handler/email_confirmation"
	"github.com/cutreapp/cutre/go/internal/handler/health"
	"github.com/cutreapp/cutre/go/internal/handler/home"
	"github.com/cutreapp/cutre/go/internal/handler/invitation_acceptance"
	"github.com/cutreapp/cutre/go/internal/handler/password"
	"github.com/cutreapp/cutre/go/internal/handler/password_reset"
	"github.com/cutreapp/cutre/go/internal/handler/password_reset_sent"
	"github.com/cutreapp/cutre/go/internal/handler/profile"
	"github.com/cutreapp/cutre/go/internal/handler/settings_invitation"
	"github.com/cutreapp/cutre/go/internal/handler/settings_two_factor_auth"
	"github.com/cutreapp/cutre/go/internal/handler/settings_withdrawal"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in_two_factor"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in_two_factor_recovery"
	"github.com/cutreapp/cutre/go/internal/handler/sign_up"
	"github.com/cutreapp/cutre/go/internal/handler/user_session"
	"github.com/cutreapp/cutre/go/internal/handler/welcome"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/turnstile"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
	"github.com/cutreapp/cutre/go/internal/worker"
)

// runServe はHTTPサーバーを起動し、シャットダウンが完了するまでブロックする。
// 戻り値はプロセスの終了コード。
func runServe() int {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定の読み込みに失敗しました", "error", err)
		return 1
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("データベース接続に失敗しました", "error", err)
		return 1
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("データベース接続のクローズに失敗しました", "error", err)
		}
	}()
	slog.Info("データベースに接続しました")

	// バックグラウンドジョブはHTTPサーバーと同じプロセスで処理する。
	workerClient, err := worker.NewClient(context.Background(), cfg, db)
	if err != nil {
		slog.Error("ワーカーの作成に失敗しました", "error", err)
		return 1
	}
	if err := workerClient.Start(context.Background()); err != nil {
		slog.Error("ワーカーの起動に失敗しました", "error", err)
		return 1
	}
	// deferは逆順に走るため、ワーカーはデータベース接続を閉じる前に止まる。
	// 処理中のジョブが接続を失わないようにするため。
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()

		if err := workerClient.Stop(stopCtx); err != nil {
			slog.Error("ワーカーの停止に失敗しました", "error", err)
		}
	}()

	// 静的アセットの配信元は、プロセスの作業ディレクトリからの相対パスで指す。
	// go/ で起動する前提のため、リポジトリのgo/staticに解決される。
	const staticDir = "./static"

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	slog.Info("サーバーを起動します", "addr", addr, "env", cfg.Env)

	srv := &http.Server{
		Addr:           addr,
		Handler:        newRouter(cfg, db, staticDir),
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// SIGINT / SIGTERMを受けたら新規接続の受け付けを止め、処理中のリクエストの完了を (タイムアウトまで) 待つ。
	shutdownDone := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("シャットダウンシグナルを受信しました")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("サーバーのシャットダウンに失敗しました", "error", err)
		}
		close(shutdownDone)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("サーバーの起動に失敗しました", "error", err)
		return 1
	}

	// ListenAndServe は Shutdown の呼び出し直後に戻るため、ドレインの完了を待ってから終了する。
	<-shutdownDone
	slog.Info("サーバーを停止しました")

	return 0
}

// staticPathPrefix は静的アセットを配信するURLのパス接頭辞。
// 配信のルートと、そこをセッションの検索・延長から外す条件の両方がこの値を使う。
// 2つは「公開キャッシュ対象の応答に利用者固有のSet-Cookieを載せない」という1つの意図で結び付いているため、
// 接頭辞を1か所に持たせて片方だけがずれないようにする。
const staticPathPrefix = "/static"

// newRouter はルーティングとミドルウェアを登録したルーターを返す。
// 具体型を返すのは、テストが登録済みのミドルウェアを通る検証用のルートを足せるようにするため。
//
// 静的アセットの配信元を引数で受け取るのは、テストが作業ディレクトリに依存せず、
// ビルド済みのアセットも要求せずに配信の配線を確認できるようにするため。
func newRouter(cfg *config.Config, db *sql.DB, staticDir string) *chi.Mux {
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)
	limiter := ratelimit.NewLimiter(repository.NewRateLimitRepository(db))
	sessionMgr := session.NewManager(userSessionRepo)
	flashMgr := session.NewFlashManager()
	authMiddleware := middleware.NewAuth(sessionMgr)
	csrfMiddleware := middleware.NewCSRF()

	healthHandler := health.NewHandler()
	welcomeHandler := welcome.NewHandler(cfg)
	homeHandler := home.NewHandler(cfg)

	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	createSignInUC := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo), userTwoFactorAuthRepo)
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)
	// ジョブを投入するクライアントが拒むのは、コードで決まる設定の誤りだけで、実行時の状態では失敗しない。
	// 誤りがあれば起動の時点で止める。
	jobDispatcher, err := dispatcher.NewDispatcher(db)
	if err != nil {
		panic(fmt.Sprintf("ジョブを投入するクライアントの作成に失敗しました: %v", err))
	}

	turnstileClient := turnstile.NewClient(cfg.TurnstileSecretKey)
	continuationMgr := session.NewContinuationManager(cfg.ContinuationTokenKey)
	invitationRepo := repository.NewInvitationRepository(db)
	invitationRedemptionRepo := repository.NewInvitationRedemptionRepository(db)
	getInvitationByIDUC := usecase.NewGetInvitationByIDUsecase(invitationRepo, invitationRedemptionRepo)
	getInvitationUC := usecase.NewGetInvitationUsecase(invitationRepo, invitationRedemptionRepo, userRepo)
	invitationAcceptanceHandler := invitation_acceptance.NewHandler(cfg, continuationMgr, limiter, getInvitationUC)
	userSessionHandler := user_session.NewHandler(sessionMgr, flashMgr, usecase.NewDeleteSessionUsecase(userSessionRepo))
	signInHandler := sign_in.NewHandler(cfg, sessionMgr, continuationMgr, flashMgr, limiter, turnstileClient, createSignInUC, createSessionUC)
	signUpHandler := sign_up.NewHandler(
		cfg,
		continuationMgr,
		limiter,
		turnstileClient,
		getInvitationByIDUC,
		usecase.NewCreateSignUpUsecase(db, invitationRepo, invitationRedemptionRepo, validator.NewSignUpCreateValidator(userRepo), repository.NewEmailConfirmationRepository(db), jobDispatcher),
	)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	emailConfirmationHandler := email_confirmation.NewHandler(
		cfg,
		continuationMgr,
		flashMgr,
		limiter,
		getInvitationByIDUC,
		usecase.NewGetEmailConfirmationUsecase(emailConfirmationRepo),
		usecase.NewVerifyEmailConfirmationUsecase(validator.NewEmailConfirmationCreateValidator(), emailConfirmationRepo),
		usecase.NewCreateSignUpUsecase(db, invitationRepo, invitationRedemptionRepo, validator.NewSignUpCreateValidator(userRepo), emailConfirmationRepo, jobDispatcher),
	)
	accountHandler := account.NewHandler(
		cfg,
		continuationMgr,
		sessionMgr,
		flashMgr,
		getInvitationByIDUC,
		usecase.NewGetConfirmedEmailConfirmationUsecase(emailConfirmationRepo),
		usecase.NewCreateAccountUsecase(
			db,
			invitationRepo,
			invitationRedemptionRepo,
			emailConfirmationRepo,
			validator.NewAccountCreateValidator(userRepo),
			userRepo,
			userPasswordRepo,
		),
		createSessionUC,
	)
	passwordResetHandler := password_reset.NewHandler(
		cfg,
		limiter,
		turnstileClient,
		usecase.NewCreatePasswordResetUsecase(validator.NewPasswordResetCreateValidator(userRepo), jobDispatcher),
	)
	passwordResetSentHandler := password_reset_sent.NewHandler(cfg)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(db)
	passwordHandler := password.NewHandler(
		cfg,
		continuationMgr,
		flashMgr,
		usecase.NewGetPasswordResetTokenUsecase(passwordResetTokenRepo),
		usecase.NewGetPasswordResetTokenByIDUsecase(passwordResetTokenRepo, userRepo),
		usecase.NewUpdatePasswordUsecase(db, passwordResetTokenRepo, validator.NewPasswordUpdateValidator(), userPasswordRepo, userSessionRepo),
	)
	errorRenderer := httperror.NewRenderer(cfg)
	getInvitationRedemptionsUC := usecase.NewGetInvitationRedemptionsUsecase(invitationRedemptionRepo)
	// 鍵の導出と暗号の準備が拒むのは、設定の読み込みが既に弾く短すぎる鍵だけで、実行時の状態では失敗しない。
	twoFactorKey, err := auth.NewTwoFactorKey(cfg.TOTPEncryptionKey)
	if err != nil {
		panic(fmt.Sprintf("二要素認証の鍵の準備に失敗しました: %v", err))
	}
	userTwoFactorRecoveryCodeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)
	getTwoFactorAuthStatusUC := usecase.NewGetTwoFactorAuthStatusUsecase(userTwoFactorAuthRepo, userTwoFactorRecoveryCodeRepo)
	profileHandler := profile.NewHandler(cfg, errorRenderer, getInvitationRedemptionsUC, getTwoFactorAuthStatusUC)
	settingsInvitationHandler := settings_invitation.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		usecase.NewPrepareInvitationUsecase(db, userRepo, invitationRepo, invitationRedemptionRepo),
		usecase.NewRecreateInvitationUsecase(db, userRepo, invitationRepo, invitationRedemptionRepo),
		getInvitationRedemptionsUC,
	)
	settingsTwoFactorAuthHandler := settings_two_factor_auth.NewHandler(
		cfg,
		flashMgr,
		limiter,
		getTwoFactorAuthStatusUC,
		usecase.NewPrepareTwoFactorAuthUsecase(twoFactorKey, userTwoFactorAuthRepo),
		usecase.NewGetPendingTwoFactorAuthUsecase(twoFactorKey, userTwoFactorAuthRepo),
		usecase.NewEnableTwoFactorAuthUsecase(
			db,
			twoFactorKey,
			validator.NewTwoFactorAuthCreateValidator(),
			userTwoFactorAuthRepo,
			userTwoFactorRecoveryCodeRepo,
		),
		usecase.NewDisableTwoFactorAuthUsecase(
			db,
			twoFactorKey,
			validator.NewTwoFactorAuthDeleteValidator(),
			userPasswordRepo,
			userTwoFactorAuthRepo,
			userTwoFactorRecoveryCodeRepo,
		),
	)
	settingsWithdrawalHandler := settings_withdrawal.NewHandler(
		cfg,
		sessionMgr,
		flashMgr,
		limiter,
		usecase.NewDeleteAccountUsecase(
			db,
			validator.NewWithdrawalDeleteValidator(userPasswordRepo),
			userRepo,
			userPasswordRepo,
			userSessionRepo,
			userTwoFactorAuthRepo,
			userTwoFactorRecoveryCodeRepo,
			passwordResetTokenRepo,
			emailConfirmationRepo,
			invitationRepo,
		),
	)

	signInTwoFactorHandler := sign_in_two_factor.NewHandler(
		cfg,
		continuationMgr,
		sessionMgr,
		flashMgr,
		limiter,
		usecase.NewCreateSignInTwoFactorUsecase(twoFactorKey, validator.NewSignInTwoFactorCreateValidator(), userRepo, userTwoFactorAuthRepo),
		createSessionUC,
	)
	signInTwoFactorRecoveryHandler := sign_in_two_factor_recovery.NewHandler(
		cfg,
		continuationMgr,
		sessionMgr,
		flashMgr,
		limiter,
		usecase.NewCreateSignInTwoFactorRecoveryUsecase(
			db,
			twoFactorKey,
			validator.NewSignInTwoFactorRecoveryCreateValidator(),
			userRepo,
			userTwoFactorAuthRepo,
			userTwoFactorRecoveryCodeRepo,
			userSessionRepo,
		),
	)

	r := chi.NewRouter()

	// セキュリティヘッダーはチェーンの先頭に置く。Recovererがpanicから返す500や、
	// 静的アセットの配信のようにハンドラーを経ないレスポンスにも載せるため。
	r.Use(middleware.SecurityHeaders)

	// chimiddleware.RealIP はGHSA-3fxj-6jh8-hvhxのため使わない。
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	// リダイレクトの発行元は、内側が書いたステータスを見てから付ける。
	// 末尾スラッシュの正規化より外側に置き、そこが出す301も覆う。
	r.Use(middleware.RedirectBy)

	// HTMLのキャッシュ方針は、応答のContent-Typeを見てから与える。
	// 末尾スラッシュの正規化より外側に置き、そこが出す301のHTMLも覆う。
	r.Use(middleware.HTMLCache)

	// 末尾スラッシュ付きのURLはスラッシュ無しの同じURLへ301で正規化する。
	// ロケールをパスから決めるI18nより外側に置き、/en/ を /en へ正規化してから判定させる。
	r.Use(middleware.RedirectSlashes)

	// HEADのルートが無いときはGETへフォールバックする。
	// 後続へはHEADのまま渡し、FileServerが本文の読み取りを省略できるようにする。
	r.Use(chimiddleware.GetHead)

	// 現在のユーザーの解決はI18nより先に置く。
	// ログイン後のページの表示言語は users.locale で決めるため、ロケールを決める側がユーザーを見られる必要がある。
	// I18nは404 / 405のページを含む全ルートに掛かっており、その外側に置ける位置は同じく全ルートしかない。
	// 静的アセットは利用者に依存しない公開キャッシュ対象のため、セッションの検索・延長を行わない。
	// 延長時のSet-Cookieを public, immutable の応答へ載せないことも、この除外で保証する。
	r.Use(chimiddleware.Maybe(authMiddleware.SetUser, outsideStaticAssets))

	r.Use(middleware.I18n)

	// CSRFの検証とトークンの発行。フォームを描くページより外側であればどこでもよいが、
	// 安全なリクエストにトークンのCookieを発行するため、SetUserと同じく静的アセットからは外す。
	r.Use(chimiddleware.Maybe(csrfMiddleware.Middleware, outsideStaticAssets))

	// _method によるメソッドの上書きはCSRFの検証より内側に置く。
	// 検証を通す前に書き換えると、上書き後のメソッドを基準に検証することになる。
	r.Use(middleware.MethodOverride)

	// フラッシュメッセージの読み取りは、応答にCookieの消去を載せる。
	// 静的アセットのリクエストで消費されると、ページが読む前にメッセージが消えるため同じく外す。
	r.Use(chimiddleware.Maybe(flashMgr.Middleware, outsideStaticAssets))

	// 静的アセットはビルド結果をディスクから読んで配信する。
	// キャッシュ方針はページと異なるため、このルートにだけAssetCacheを重ねる。
	fileServer := http.FileServer(assetFileSystem{http.Dir(staticDir)})
	r.With(middleware.AssetCache(cfg)).Handle(staticPathPrefix+"/*", http.StripPrefix(staticPathPrefix, fileServer))

	r.Get("/health", healthHandler.Show)

	// 公開ページの言語版はURLで表す。日本語版は言語コードを持たず、英語版だけが /en 配下に来る。
	// トップページはログイン前の訪問者向けのため、ログイン済みの訪問者はホームへ送る。
	r.With(authMiddleware.RequireNoAuth).Get("/", welcomeHandler.Show)
	r.With(authMiddleware.RequireNoAuth).Get("/en", welcomeHandler.Show)

	// ログイン画面もログイン前の公開ページのため、言語版をURLで持つ。
	// 資格情報を入力するフォームはHTTPキャッシュに保存させない。BFCacheから復元された入力内容は画面側で消す。
	r.Group(func(r chi.Router) {
		r.Use(middleware.NoStore, authMiddleware.RequireNoAuth)
		for _, lang := range i18n.SupportedLangs() {
			path := i18n.LocalePath(lang, templates.SignInPath)
			r.Get(path, signInHandler.New)
			r.Post(path, signInHandler.Create)

			// パスワードを確かめた二要素認証のユーザーが、認証アプリのコードかリカバリーコードを入力する。ログイン画面の言語版のまま進む。
			signInTwoFactorPath := i18n.LocalePath(lang, templates.SignInTwoFactorPath)
			r.Get(signInTwoFactorPath, signInTwoFactorHandler.New)
			r.Post(signInTwoFactorPath, signInTwoFactorHandler.Create)
			signInTwoFactorRecoveryPath := i18n.LocalePath(lang, templates.SignInTwoFactorRecoveryPath)
			r.Get(signInTwoFactorRecoveryPath, signInTwoFactorRecoveryHandler.New)
			r.Post(signInTwoFactorRecoveryPath, signInTwoFactorRecoveryHandler.Create)

			// 登録の画面もログイン前の公開ページで、メールアドレスを入力するフォームを持つ。
			signUpPath := i18n.LocalePath(lang, templates.SignUpPath)
			r.Get(signUpPath, signUpHandler.New)
			r.Post(signUpPath, signUpHandler.Create)

			confirmationPath := i18n.LocalePath(lang, templates.EmailConfirmationPath)
			r.Get(confirmationPath, emailConfirmationHandler.New)
			r.Post(confirmationPath, emailConfirmationHandler.Create)
			r.Patch(confirmationPath, emailConfirmationHandler.Update)

			accountPath := i18n.LocalePath(lang, templates.AccountPath)
			r.Get(accountPath, accountHandler.New)
			r.Post(accountPath, accountHandler.Create)

			// パスワードリセットの申請もログイン前の画面で、メールアドレスを入力するフォームを持つ。
			passwordResetPath := i18n.LocalePath(lang, templates.PasswordResetPath)
			r.Get(passwordResetPath, passwordResetHandler.New)
			r.Post(passwordResetPath, passwordResetHandler.Create)
			r.Get(i18n.LocalePath(lang, templates.PasswordResetSentPath), passwordResetSentHandler.Show)
		}
	})

	// 招待やパスワードリセットのトークンを含むURLを参照元として送らせず、認証済み利用者へのリダイレクトにも適用する。
	r.Group(func(r chi.Router) {
		r.Use(middleware.NoReferrer, middleware.NoStore, authMiddleware.RequireNoAuth)
		for _, lang := range i18n.SupportedLangs() {
			path := i18n.LocalePath(lang, templates.InvitationPath("{token}"))
			r.Get(path, invitationAcceptanceHandler.New)
			r.Post(path, invitationAcceptanceHandler.Create)

			// パスワードリセットのメールのリンク (?token=) で開き、トークンをCookieへ移してからフォームを描画する。
			passwordPath := i18n.LocalePath(lang, templates.PasswordPath)
			r.Get(passwordPath, passwordHandler.Edit)
			r.Patch(passwordPath, passwordHandler.Update)
		}
	})

	// ログアウトはログインを求めない。未ログインでのログアウトも同じ行き先へ送り、何度行っても同じ結果にする。
	r.Delete(templates.UserSessionPath, userSessionHandler.Delete)

	// ログイン後のページは言語版のURLを持たず、users.locale の言語で表示する。
	// ログイン後のページはHTTPキャッシュに保存させない。
	r.Group(func(r chi.Router) {
		r.Use(middleware.NoStore, authMiddleware.RequireAuth, middleware.UserLocale)
		r.Get(templates.HomePath, homeHandler.Show)
		r.Get(templates.ProfilePath("{atname}"), profileHandler.Show)
		r.Get(templates.SettingsInvitationPath, settingsInvitationHandler.Show)
		r.Post(templates.SettingsInvitationPath, settingsInvitationHandler.Create)
		r.Get(templates.SettingsTwoFactorAuthPath, settingsTwoFactorAuthHandler.Show)
		r.Get(templates.NewSettingsTwoFactorAuthPath, settingsTwoFactorAuthHandler.New)
		r.Post(templates.SettingsTwoFactorAuthPath, settingsTwoFactorAuthHandler.Create)
		r.Delete(templates.SettingsTwoFactorAuthPath, settingsTwoFactorAuthHandler.Delete)
		r.Get(templates.SettingsWithdrawalPath, settingsWithdrawalHandler.New)
		r.Delete(templates.SettingsWithdrawalPath, settingsWithdrawalHandler.Delete)
	})

	// どのルートにも一致しないリクエストは共通の404ページで応える。
	// 登録しないとchiの既定の平文1行が返り、そこから先へ進む手段が無い。
	r.NotFound(errorRenderer.NotFound)

	// アドレスはあるがそのメソッドを受け付けないリクエストは共通の405ページで応える。
	// Allowを自分で付けるのは、ハンドラーを差し替えるとchiが許可メソッドの一覧をハンドラーへ渡さなくなるため。
	// RFC 9110は405の応答にこのヘッダーを求めており、差し替えで落とすとchiの既定より情報が減る。
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		for _, method := range allowedMethods(r, req.URL.Path) {
			w.Header().Add("Allow", method)
		}

		errorRenderer.MethodNotAllowed(w, req)
	})

	return r
}

// outsideStaticAssets は静的アセット以外のリクエストかどうかを返す。
//
// 利用者ごとに値が変わるミドルウェア (セッション・CSRF・フラッシュ) を掛ける範囲をこれで決める。
// 静的アセットの応答は公開キャッシュの対象で、利用者固有のSet-Cookieを載せられない。
func outsideStaticAssets(r *http.Request) bool {
	return !strings.HasPrefix(r.URL.Path, staticPathPrefix+"/")
}

// assetFileSystem は包んだファイルシステムのディレクトリを存在しないものとして扱う。
//
// http.FileServer はディレクトリを開けたとき、URLが末尾スラッシュを持たなければ
// それを足すリダイレクトを返す。それは middleware.RedirectSlashes が剥がすリダイレクトそのもので、
// 2つを組み合わせると訪問者が /static/css/ と /static/css の間を往復し続ける。
// 末尾スラッシュ付きで届いた場合も、返るのは収めたファイル名を並べた一覧であり、
// アセットのバージョンを持たないURLから配ることになる。
// ディレクトリを開けなくすれば、どちらの形も404に落ち着く。
type assetFileSystem struct {
	http.FileSystem
}

// Open はファイルだけを返し、ディレクトリには fs.ErrNotExist を返す。
func (fsys assetFileSystem) Open(name string) (http.File, error) {
	file, err := fsys.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}

	if info.IsDir() {
		return nil, errors.Join(fs.ErrNotExist, file.Close())
	}

	return file, nil
}

// allowProbeMethods はAllowヘッダーを組み立てるとき、登録の有無をルーターに問い合わせるHTTPメソッド。
// chiのルーターが解釈できるメソッドを並べる。
var allowProbeMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodConnect,
	http.MethodOptions,
	http.MethodTrace,
}

// allowedMethods はpathに登録されているメソッドを、Allowヘッダーに出す順で返す。
// 登録の有無はルーター自身に問い合わせるため、ルートを足してもここを書き換える必要はない。
func allowedMethods(router *chi.Mux, path string) []string {
	allowed := make([]string, 0, len(allowProbeMethods))

	for _, method := range allowProbeMethods {
		matches := router.Match(chi.NewRouteContext(), method, path)
		// GetHeadのフォールバックをAllowにも反映する。
		// 明示したHEADのルートはGETの有無にかかわらず受け付ける。
		if !matches && method == http.MethodHead {
			matches = router.Match(chi.NewRouteContext(), http.MethodGet, path)
		}
		if matches {
			allowed = append(allowed, method)
		}
	}

	return allowed
}
