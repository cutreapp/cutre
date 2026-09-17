package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/database"
	"github.com/cutreapp/cutre/go/internal/handler/health"
	"github.com/cutreapp/cutre/go/internal/handler/welcome"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/middleware"
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

	// 静的アセットの配信元は、プロセスの作業ディレクトリからの相対パスで指す。
	// go/ で起動する前提のため、リポジトリのgo/staticに解決される。
	const staticDir = "./static"

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	slog.Info("サーバーを起動します", "addr", addr, "env", cfg.Env)

	srv := &http.Server{
		Addr:           addr,
		Handler:        newRouter(cfg, staticDir),
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

// newRouter はルーティングとミドルウェアを登録したルーターを返す。
// 具体型を返すのは、テストが登録済みのミドルウェアを通る検証用のルートを足せるようにするため。
//
// 静的アセットの配信元を引数で受け取るのは、テストが作業ディレクトリに依存せず、
// ビルド済みのアセットも要求せずに配信の配線を確認できるようにするため。
func newRouter(cfg *config.Config, staticDir string) *chi.Mux {
	healthHandler := health.NewHandler()
	welcomeHandler := welcome.NewHandler(cfg)
	errorRenderer := httperror.NewRenderer(cfg)

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

	r.Use(middleware.I18n)

	// 静的アセットはビルド結果をディスクから読んで配信する。
	// キャッシュ方針はページと異なるため、このルートにだけAssetCacheを重ねる。
	fileServer := http.FileServer(assetFileSystem{http.Dir(staticDir)})
	r.With(middleware.AssetCache(cfg)).Handle("/static/*", http.StripPrefix("/static", fileServer))

	r.Get("/health", healthHandler.Show)

	// 公開ページの言語版はURLで表す。日本語版は言語コードを持たず、英語版だけが /en 配下に来る。
	r.Get("/", welcomeHandler.Show)
	r.Get("/en", welcomeHandler.Show)

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
