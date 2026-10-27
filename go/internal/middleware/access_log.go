package middleware

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// AccessLog はリクエストごとに1件のアクセスログをslogで出力する。
//
// パスではなくルートパターンを出すのは、パスに含まれるトークン (確認やリセットのURLなど) をログに残さず、
// ルートごとの集計もしやすくするため。
// Cf-Ray はCloudflare側のログと突き合わせるために残す。
//
// リクエストIDはcontextから基底のハンドラーが加えるため、RequestIDより内側に登録する。
// Recovererより外側に登録し、panicから返す500も記録する。
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(lw, r)

		// ルートパターンはルーティングを終えたあとでないと決まらない。
		// 一致するルートが無い (404など) ときは空になる。
		var route string
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			route = rctx.RoutePattern()
		}

		slog.InfoContext(r.Context(), "リクエストを処理しました",
			"method", r.Method,
			"route", route,
			"status", lw.statusCode(),
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
			"cf_ray", r.Header.Get("Cf-Ray"),
		)
	})
}

// statusRecorder は内側が書き出したステータスを記録する。
// アクセスログ (AccessLog) と、panic時のSentryのトレースの補正 (SentryTransaction) の両方から使うため、
// 記録の仕方を変えるとどちらの出力にも波及する。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	// 1xxの暫定応答のあとにも最終的なステータスが続くため、記録するのは最終的なものだけにする。
	if w.status == 0 && status >= http.StatusOK {
		w.status = status
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return w.ResponseWriter.Write(b)
}

// statusCode はクライアントへ送ったステータスを返す。
// 内側が何も書かずに戻ったときは、net/httpが200を送るため200を返す。
func (w *statusRecorder) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}

	return w.status
}

// FlushError はFlushによる暗黙の200も記録し、後続のエラー応答で上書きされるのを防ぐ。
func (w *statusRecorder) FlushError() error {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if errors.Is(err, http.ErrNotSupported) {
		// 非対応なら何も送出していないため、後続のWriteHeaderで確定できるようにする。
		return err
	}

	// 書き出しに失敗しても、下層ではヘッダーの送出を確定している。
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return err
}

// Unwrap は下層のResponseWriterを返す。
// http.ResponseController が、サーバー自身のResponseWriterが実装する追加のインターフェース
// (Flusher・Hijacker・デッドラインの設定など) へ到達するための手段である。
func (w *statusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// ReadFrom は本文の書き出しを下層のResponseWriterに任せる。
// 静的アセットの配信がsendfileを使えるよう、埋め込みでは昇格しない io.ReaderFrom を橋渡しする。
func (w *statusRecorder) ReadFrom(r io.Reader) (int64, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	return io.Copy(w.ResponseWriter, r)
}
