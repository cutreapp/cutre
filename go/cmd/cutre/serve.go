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
	"github.com/cutreapp/cutre/go/internal/handler/admin"
	"github.com/cutreapp/cutre/go/internal/handler/admin_event"
	"github.com/cutreapp/cutre/go/internal/handler/admin_event_archive"
	"github.com/cutreapp/cutre/go/internal/handler/admin_event_category"
	"github.com/cutreapp/cutre/go/internal/handler/admin_event_category_archive"
	"github.com/cutreapp/cutre/go/internal/handler/admin_goods"
	"github.com/cutreapp/cutre/go/internal/handler/admin_goods_archive"
	"github.com/cutreapp/cutre/go/internal/handler/admin_station"
	"github.com/cutreapp/cutre/go/internal/handler/admin_station_archive"
	"github.com/cutreapp/cutre/go/internal/handler/email_confirmation"
	"github.com/cutreapp/cutre/go/internal/handler/event"
	"github.com/cutreapp/cutre/go/internal/handler/event_category"
	"github.com/cutreapp/cutre/go/internal/handler/health"
	"github.com/cutreapp/cutre/go/internal/handler/home"
	"github.com/cutreapp/cutre/go/internal/handler/invitation_acceptance"
	"github.com/cutreapp/cutre/go/internal/handler/item"
	"github.com/cutreapp/cutre/go/internal/handler/list"
	"github.com/cutreapp/cutre/go/internal/handler/match"
	"github.com/cutreapp/cutre/go/internal/handler/message"
	"github.com/cutreapp/cutre/go/internal/handler/password"
	"github.com/cutreapp/cutre/go/internal/handler/password_reset"
	"github.com/cutreapp/cutre/go/internal/handler/password_reset_sent"
	"github.com/cutreapp/cutre/go/internal/handler/profile"
	"github.com/cutreapp/cutre/go/internal/handler/settings_invitation"
	"github.com/cutreapp/cutre/go/internal/handler/settings_message_consent"
	"github.com/cutreapp/cutre/go/internal/handler/settings_place"
	"github.com/cutreapp/cutre/go/internal/handler/settings_two_factor_auth"
	"github.com/cutreapp/cutre/go/internal/handler/settings_withdrawal"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in_two_factor"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in_two_factor_recovery"
	"github.com/cutreapp/cutre/go/internal/handler/sign_up"
	"github.com/cutreapp/cutre/go/internal/handler/trade"
	"github.com/cutreapp/cutre/go/internal/handler/trade_approval"
	"github.com/cutreapp/cutre/go/internal/handler/trade_cancellation"
	"github.com/cutreapp/cutre/go/internal/handler/trade_completion"
	"github.com/cutreapp/cutre/go/internal/handler/trade_confirmation"
	"github.com/cutreapp/cutre/go/internal/handler/trade_decline"
	"github.com/cutreapp/cutre/go/internal/handler/trade_failure"
	"github.com/cutreapp/cutre/go/internal/handler/trade_history"
	"github.com/cutreapp/cutre/go/internal/handler/trade_message"
	"github.com/cutreapp/cutre/go/internal/handler/trade_message_retraction"
	"github.com/cutreapp/cutre/go/internal/handler/trade_withdrawal"
	"github.com/cutreapp/cutre/go/internal/handler/user_session"
	"github.com/cutreapp/cutre/go/internal/handler/welcome"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
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
	messageConsentRepo := repository.NewMessageConsentRepository(db)
	tradeRepo := repository.NewTradeRepository(db)
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
			messageConsentRepo,
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
	getMessageConsentUC := usecase.NewGetMessageConsentUsecase(messageConsentRepo)
	stationRepo := repository.NewStationRepository(db)
	userStationRepo := repository.NewUserStationRepository(db)
	getPlacesUC := usecase.NewGetPlacesUsecase(stationRepo, userRepo)
	settingsPlaceHandler := settings_place.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getPlacesUC,
		usecase.NewGetPublishedStationsUsecase(stationRepo),
		usecase.NewUpdatePlacesUsecase(
			db,
			validator.NewPlaceUpdateValidator(),
			validator.NewPlaceStationUpdateValidator(stationRepo),
			stationRepo,
			userStationRepo,
			userRepo,
		),
	)
	settingsMessageConsentHandler := settings_message_consent.NewHandler(
		cfg,
		flashMgr,
		getMessageConsentUC,
		usecase.NewCreateMessageConsentUsecase(messageConsentRepo),
		usecase.NewWithdrawMessageConsentUsecase(db, validator.NewMessageConsentWithdrawValidator(tradeRepo), messageConsentRepo, userRepo),
	)
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
	itemRepo := repository.NewItemRepository(db)
	settingsWithdrawalHandler := settings_withdrawal.NewHandler(
		cfg,
		sessionMgr,
		flashMgr,
		limiter,
		usecase.NewGetWithdrawalUsecase(tradeRepo),
		usecase.NewDeleteAccountUsecase(
			db,
			validator.NewWithdrawalDeleteValidator(userPasswordRepo, tradeRepo),
			userRepo,
			userPasswordRepo,
			userSessionRepo,
			userTwoFactorAuthRepo,
			userTwoFactorRecoveryCodeRepo,
			passwordResetTokenRepo,
			emailConfirmationRepo,
			invitationRepo,
			itemRepo,
			userStationRepo,
		),
	)

	eventRepo := repository.NewEventRepository(db)
	eventCategoryRepo := repository.NewEventCategoryRepository(db)
	goodsRepo := repository.NewGoodsRepository(db)
	profileHandler := profile.NewHandler(
		cfg,
		errorRenderer,
		getInvitationRedemptionsUC,
		getTwoFactorAuthStatusUC,
		getMessageConsentUC,
		getPlacesUC,
		usecase.NewGetProfileUsecase(eventCategoryRepo, goodsRepo, itemRepo, stationRepo, tradeRepo, userRepo),
		usecase.NewGetEndedTradeCountsUsecase(tradeRepo),
	)

	// 管理画面を使えるかは各UseCaseが役割で確かめ、使えないユーザーにはハンドラーが404を返す。
	getAdminMenuUC := usecase.NewGetAdminMenuUsecase()
	getAdminEventUC := usecase.NewGetAdminEventUsecase(eventRepo, eventCategoryRepo)
	getAdminEventCategoryUC := usecase.NewGetAdminEventCategoryUsecase(eventRepo, eventCategoryRepo, goodsRepo)
	getAdminGoodsUC := usecase.NewGetAdminGoodsUsecase(eventRepo, eventCategoryRepo, goodsRepo)
	adminHandler := admin.NewHandler(cfg, errorRenderer, getAdminMenuUC)
	adminEventHandler := admin_event.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getAdminMenuUC,
		usecase.NewGetAdminEventsUsecase(eventRepo),
		getAdminEventUC,
		usecase.NewCreateEventUsecase(validator.NewEventCreateValidator(), eventRepo),
		usecase.NewUpdateEventUsecase(validator.NewEventUpdateValidator(), eventRepo),
		usecase.NewDeleteEventUsecase(db, validator.NewEventDeleteValidator(itemRepo), eventRepo),
	)
	adminEventArchiveHandler := admin_event_archive.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getAdminEventUC,
		usecase.NewArchiveEventUsecase(validator.NewEventArchiveCreateValidator(), eventRepo),
		usecase.NewUnarchiveEventUsecase(eventRepo),
	)
	adminEventCategoryHandler := admin_event_category.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getAdminEventUC,
		getAdminEventCategoryUC,
		usecase.NewCreateEventCategoryUsecase(validator.NewEventCategoryCreateValidator(), eventRepo, eventCategoryRepo),
		usecase.NewUpdateEventCategoryUsecase(validator.NewEventCategoryUpdateValidator(), eventRepo, eventCategoryRepo),
		usecase.NewDeleteEventCategoryUsecase(db, validator.NewEventCategoryDeleteValidator(itemRepo), eventRepo, eventCategoryRepo),
	)
	adminEventCategoryArchiveHandler := admin_event_category_archive.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getAdminEventCategoryUC,
		usecase.NewArchiveEventCategoryUsecase(validator.NewEventCategoryArchiveCreateValidator(), eventRepo, eventCategoryRepo),
		usecase.NewUnarchiveEventCategoryUsecase(eventRepo, eventCategoryRepo),
	)
	adminGoodsHandler := admin_goods.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getAdminEventCategoryUC,
		getAdminGoodsUC,
		usecase.NewCreateGoodsUsecase(validator.NewGoodsCreateValidator(), eventRepo, eventCategoryRepo, goodsRepo),
		usecase.NewUpdateGoodsUsecase(validator.NewGoodsUpdateValidator(), eventRepo, eventCategoryRepo, goodsRepo),
		usecase.NewDeleteGoodsUsecase(db, validator.NewGoodsDeleteValidator(itemRepo), eventRepo, eventCategoryRepo, goodsRepo),
	)
	adminGoodsArchiveHandler := admin_goods_archive.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getAdminGoodsUC,
		usecase.NewArchiveGoodsUsecase(validator.NewGoodsArchiveCreateValidator(), eventRepo, eventCategoryRepo, goodsRepo),
		usecase.NewUnarchiveGoodsUsecase(eventRepo, eventCategoryRepo, goodsRepo),
	)
	eventHandler := event.NewHandler(
		cfg,
		errorRenderer,
		usecase.NewGetEventsUsecase(eventRepo, goodsRepo, itemRepo),
		usecase.NewGetEventUsecase(eventRepo, eventCategoryRepo, goodsRepo, itemRepo),
	)
	eventCategoryHandler := event_category.NewHandler(cfg, errorRenderer, usecase.NewGetEventCategoryUsecase(eventRepo, eventCategoryRepo, goodsRepo, itemRepo))
	itemHandler := item.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		usecase.NewGetGoodsUsecase(eventRepo, eventCategoryRepo, goodsRepo),
		usecase.NewCreateItemUsecase(db, validator.NewItemCreateValidator(), eventRepo, eventCategoryRepo, goodsRepo, itemRepo),
		usecase.NewGetItemUsecase(eventRepo, eventCategoryRepo, goodsRepo, itemRepo),
		usecase.NewUpdateItemUsecase(validator.NewItemUpdateValidator(), itemRepo),
		usecase.NewDeleteItemUsecase(itemRepo),
	)
	listHandler := list.NewHandler(cfg, usecase.NewGetListUsecase(eventRepo, eventCategoryRepo, goodsRepo, itemRepo))
	homeHandler := home.NewHandler(cfg, usecase.NewGetHomeUsecase(itemRepo, userRepo, userStationRepo))
	getMatchesUC := usecase.NewGetMatchesUsecase(eventCategoryRepo, goodsRepo, itemRepo, stationRepo, userRepo, userStationRepo)
	matchHandler := match.NewHandler(cfg, getMatchesUC)
	getTradeProposalUC := usecase.NewGetTradeProposalUsecase(eventCategoryRepo, goodsRepo, itemRepo, messageConsentRepo, userRepo)
	tradeEventRepo := repository.NewTradeEventRepository(db)
	tradeItemRepo := repository.NewTradeItemRepository(db)
	tradeMessageRepo := repository.NewTradeMessageRepository(db)
	tradeMessageReadRepo := repository.NewTradeMessageReadRepository(db)
	getTradeUC := usecase.NewGetTradeUsecase(eventCategoryRepo, goodsRepo, itemRepo, messageConsentRepo, tradeRepo, tradeEventRepo, tradeItemRepo, tradeMessageRepo, userRepo)
	tradeHandler := trade.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		limiter,
		usecase.NewGetTradesUsecase(itemRepo, tradeRepo, tradeItemRepo, userRepo),
		getMatchesUC,
		getTradeUC,
		getTradeProposalUC,
		usecase.NewCreateTradeUsecase(
			db,
			validator.NewTradeCreateValidator(itemRepo),
			messageConsentRepo,
			tradeRepo,
			tradeEventRepo,
			tradeItemRepo,
			tradeMessageRepo,
			userRepo,
		),
	)
	tradeHistoryHandler := trade_history.NewHandler(cfg, usecase.NewGetTradeHistoryUsecase(itemRepo, tradeRepo, tradeItemRepo, userRepo))
	tradeConfirmationHandler := trade_confirmation.NewHandler(cfg, errorRenderer, getTradeProposalUC)
	tradeWithdrawalHandler := trade_withdrawal.NewHandler(errorRenderer, flashMgr, usecase.NewWithdrawTradeUsecase(db, tradeRepo, tradeEventRepo))
	tradeApprovalHandler := trade_approval.NewHandler(errorRenderer, flashMgr, usecase.NewApproveTradeUsecase(db, messageConsentRepo, tradeRepo, tradeEventRepo))
	tradeDeclineHandler := trade_decline.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getTradeUC,
		usecase.NewDeclineTradeUsecase(db, validator.NewTradeDeclineValidator(), messageConsentRepo, tradeRepo, tradeEventRepo, tradeMessageRepo),
	)
	tradeCompletionHandler := trade_completion.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getTradeUC,
		usecase.NewCompleteTradeUsecase(db, validator.NewTradeCompletionValidator(), itemRepo, messageConsentRepo, tradeRepo, tradeEventRepo, tradeMessageRepo),
	)
	tradeFailureHandler := trade_failure.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getTradeUC,
		usecase.NewFailTradeUsecase(db, validator.NewTradeFailureValidator(), messageConsentRepo, tradeRepo, tradeEventRepo, tradeMessageRepo),
	)
	tradeCancellationHandler := trade_cancellation.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getTradeUC,
		usecase.NewCancelTradeUsecase(db, validator.NewTradeCancellationValidator(), messageConsentRepo, tradeRepo, tradeEventRepo, tradeMessageRepo),
	)
	tradeMessageHandler := trade_message.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		limiter,
		usecase.NewGetTradeMessagesUsecase(itemRepo, messageConsentRepo, tradeRepo, tradeEventRepo, tradeItemRepo, tradeMessageRepo, tradeMessageReadRepo, userRepo),
		usecase.NewMarkTradeMessagesReadUsecase(tradeRepo, tradeMessageReadRepo),
		usecase.NewCreateTradeMessageUsecase(validator.NewTradeMessageCreateValidator(), messageConsentRepo, tradeRepo, tradeMessageRepo),
	)
	tradeMessageRetractionHandler := trade_message_retraction.NewHandler(errorRenderer, flashMgr, usecase.NewRetractTradeMessageUsecase(tradeRepo, tradeMessageRepo))
	messageHandler := message.NewHandler(cfg, usecase.NewGetMessagesUsecase(itemRepo, tradeRepo, tradeItemRepo, tradeMessageRepo, userRepo))
	// メインメニューの数字は、ログイン後のページのすべてで出すため、ルートのグループに掛けたミドルウェアが引く。
	getMainNavUC := usecase.NewGetMainNavUsecase(tradeRepo, tradeMessageRepo)
	mainNavBadges := middleware.NewMainNavBadges(func(ctx context.Context, userID model.UserID) (templates.MainNavBadges, error) {
		output, err := getMainNavUC.Execute(ctx, usecase.GetMainNavInput{UserID: userID})
		if err != nil {
			return templates.MainNavBadges{}, err
		}

		return templates.MainNavBadges{AwaitingTradeCount: output.AwaitingTradeCount, UnreadMessageCount: output.UnreadMessageCount}, nil
	})
	getAdminStationUC := usecase.NewGetAdminStationUsecase(stationRepo)
	adminStationHandler := admin_station.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		usecase.NewGetAdminStationsUsecase(stationRepo),
		getAdminStationUC,
		usecase.NewCreateStationUsecase(validator.NewStationCreateValidator(), stationRepo),
		usecase.NewUpdateStationUsecase(validator.NewStationUpdateValidator(), stationRepo),
		usecase.NewDeleteStationUsecase(db, validator.NewStationDeleteValidator(userStationRepo), stationRepo),
	)
	adminStationArchiveHandler := admin_station_archive.NewHandler(
		cfg,
		errorRenderer,
		flashMgr,
		getAdminStationUC,
		usecase.NewArchiveStationUsecase(validator.NewStationArchiveCreateValidator(), stationRepo),
		usecase.NewUnarchiveStationUsecase(stationRepo),
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
	// メインメニューに出す数字 (返事や確認を待っている交換の数) は、ここで1回引いてページに渡す。
	r.Group(func(r chi.Router) {
		r.Use(middleware.NoStore, authMiddleware.RequireAuth, middleware.UserLocale, mainNavBadges.Middleware)
		r.Get(templates.HomePath, homeHandler.Show)
		r.Get(templates.ProfilePath("{atname}"), profileHandler.Show)
		r.Get(templates.SettingsInvitationPath, settingsInvitationHandler.Show)
		r.Post(templates.SettingsInvitationPath, settingsInvitationHandler.Create)
		r.Get(templates.SettingsTwoFactorAuthPath, settingsTwoFactorAuthHandler.Show)
		r.Get(templates.NewSettingsTwoFactorAuthPath, settingsTwoFactorAuthHandler.New)
		r.Post(templates.SettingsTwoFactorAuthPath, settingsTwoFactorAuthHandler.Create)
		r.Delete(templates.SettingsTwoFactorAuthPath, settingsTwoFactorAuthHandler.Delete)
		r.Get(templates.SettingsMessageConsentPath, settingsMessageConsentHandler.Show)
		r.Post(templates.SettingsMessageConsentPath, settingsMessageConsentHandler.Create)
		r.Delete(templates.SettingsMessageConsentPath, settingsMessageConsentHandler.Delete)
		r.Get(templates.SettingsPlacesPath, settingsPlaceHandler.Show)
		r.Patch(templates.SettingsPlacesPath, settingsPlaceHandler.Update)
		r.Get(templates.SettingsWithdrawalPath, settingsWithdrawalHandler.New)
		r.Delete(templates.SettingsWithdrawalPath, settingsWithdrawalHandler.Delete)

		// リストに追加するグッズを、イベント → カテゴリー → グッズの順にたどる。公開中のマスタだけを出す。
		r.Get(templates.EventsPath, eventHandler.Index)
		r.Get(templates.EventPath("{event_id}"), eventHandler.Show)
		r.Get(templates.EventCategoryPath("{event_id}", "{category_id}"), eventCategoryHandler.Show)
		r.Get(templates.NewItemPath, itemHandler.New)
		r.Post(templates.ItemsPath, itemHandler.Create)

		// マッチ候補。交換場所の都道府県が同じで、おたがいのほしいリストに相手の譲れるアイテムがある人を出す。
		r.Get(templates.MatchesPath, matchHandler.Index)

		// 交換の申し込み。組み合わせを選び、申し込み内容を確かめてから申し込む。
		r.Get(templates.NewTradePath("{atname}"), tradeHandler.New)
		r.Get(templates.NewTradeConfirmationPath("{atname}"), tradeConfirmationHandler.New)
		r.Post(templates.TradesPath, tradeHandler.Create)

		// 交換の画面と交換のページ・交換のメッセージ。交換のページ・メッセージとその操作は、交換の2人以外には存在しないページとして404を返す。
		r.Get(templates.TradesPath, tradeHandler.Index)
		r.Get(templates.TradeHistoryPath, tradeHistoryHandler.Index)
		r.Get(templates.TradePath("{id}"), tradeHandler.Show)
		r.Post(templates.TradeWithdrawalPath("{id}"), tradeWithdrawalHandler.Create)
		r.Post(templates.TradeApprovalPath("{id}"), tradeApprovalHandler.Create)
		r.Get(templates.TradeDeclinePath("{id}"), tradeDeclineHandler.New)
		r.Post(templates.TradeDeclinePath("{id}"), tradeDeclineHandler.Create)
		r.Get(templates.TradeCompletionPath("{id}"), tradeCompletionHandler.New)
		r.Post(templates.TradeCompletionPath("{id}"), tradeCompletionHandler.Create)
		r.Get(templates.TradeFailurePath("{id}"), tradeFailureHandler.New)
		r.Post(templates.TradeFailurePath("{id}"), tradeFailureHandler.Create)
		r.Get(templates.TradeCancellationPath("{id}"), tradeCancellationHandler.New)
		r.Post(templates.TradeCancellationPath("{id}"), tradeCancellationHandler.Create)
		r.Get(templates.TradeMessagesPath("{id}"), tradeMessageHandler.Index)
		r.Post(templates.TradeMessagesPath("{id}"), tradeMessageHandler.Create)
		r.Post(templates.TradeMessageRetractionPath("{id}", "{message_id}"), tradeMessageRetractionHandler.Create)

		// メッセージの一覧。交換ごとのメッセージを、最新のメッセージが新しい順に出す。
		r.Get(templates.MessagesPath, messageHandler.Index)

		// 譲れる・ほしいのリストと、リストにあるアイテムの編集・リストから外す操作。
		r.Get(templates.ListPath, listHandler.Index)
		r.Get(templates.EditItemPath("{id}"), itemHandler.Edit)
		r.Patch(templates.ItemPath("{id}"), itemHandler.Update)
		r.Delete(templates.ItemPath("{id}"), itemHandler.Delete)

		// 管理画面。編集者と管理者だけが使え、それ以外の人には存在しないページとして404を返す。
		r.Get(templates.AdminPath, adminHandler.Show)
		r.Get(templates.AdminEventsPath, adminEventHandler.Index)
		r.Get(templates.NewAdminEventPath, adminEventHandler.New)
		r.Post(templates.AdminEventsPath, adminEventHandler.Create)
		r.Get(templates.EditAdminEventPath("{id}"), adminEventHandler.Edit)
		r.Patch(templates.AdminEventPath("{id}"), adminEventHandler.Update)
		r.Delete(templates.AdminEventPath("{id}"), adminEventHandler.Delete)
		r.Get(templates.NewAdminEventArchivePath("{id}"), adminEventArchiveHandler.New)
		r.Post(templates.AdminEventArchivePath("{id}"), adminEventArchiveHandler.Create)
		r.Delete(templates.AdminEventArchivePath("{id}"), adminEventArchiveHandler.Delete)
		// カテゴリーとグッズの作成は親 (イベント・カテゴリー) のパスの下に置き、作成したあとはそれぞれのIDだけのパスで扱う。
		r.Get(templates.NewAdminEventCategoryPath("{id}"), adminEventCategoryHandler.New)
		r.Post(templates.AdminEventCategoriesPath("{id}"), adminEventCategoryHandler.Create)
		r.Get(templates.EditAdminEventCategoryPath("{id}"), adminEventCategoryHandler.Edit)
		r.Patch(templates.AdminEventCategoryPath("{id}"), adminEventCategoryHandler.Update)
		r.Delete(templates.AdminEventCategoryPath("{id}"), adminEventCategoryHandler.Delete)
		r.Get(templates.NewAdminEventCategoryArchivePath("{id}"), adminEventCategoryArchiveHandler.New)
		r.Post(templates.AdminEventCategoryArchivePath("{id}"), adminEventCategoryArchiveHandler.Create)
		r.Delete(templates.AdminEventCategoryArchivePath("{id}"), adminEventCategoryArchiveHandler.Delete)
		r.Get(templates.NewAdminGoodsPath("{id}"), adminGoodsHandler.New)
		r.Post(templates.AdminEventCategoryGoodsPath("{id}"), adminGoodsHandler.Create)
		r.Get(templates.EditAdminGoodsPath("{id}"), adminGoodsHandler.Edit)
		r.Patch(templates.AdminGoodsPath("{id}"), adminGoodsHandler.Update)
		r.Delete(templates.AdminGoodsPath("{id}"), adminGoodsHandler.Delete)
		r.Get(templates.NewAdminGoodsArchivePath("{id}"), adminGoodsArchiveHandler.New)
		r.Post(templates.AdminGoodsArchivePath("{id}"), adminGoodsArchiveHandler.Create)
		r.Delete(templates.AdminGoodsArchivePath("{id}"), adminGoodsArchiveHandler.Delete)
		r.Get(templates.AdminStationsPath, adminStationHandler.Index)
		r.Get(templates.NewAdminStationPath, adminStationHandler.New)
		r.Post(templates.AdminStationsPath, adminStationHandler.Create)
		r.Get(templates.EditAdminStationPath("{id}"), adminStationHandler.Edit)
		r.Patch(templates.AdminStationPath("{id}"), adminStationHandler.Update)
		r.Delete(templates.AdminStationPath("{id}"), adminStationHandler.Delete)
		r.Get(templates.NewAdminStationArchivePath("{id}"), adminStationArchiveHandler.New)
		r.Post(templates.AdminStationArchivePath("{id}"), adminStationArchiveHandler.Create)
		r.Delete(templates.AdminStationArchivePath("{id}"), adminStationArchiveHandler.Delete)
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
