package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/metrics"
)

// Metrics はリクエストごとにルートパターン・メソッド・ステータスと所要時間を記録する。
//
// AccessLogと同じく、パスではなくルートパターンで数える。
// Recovererより外側に登録し、panicから返す500も数える。
func Metrics(m *metrics.HTTP) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusRecorder{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			// ルートパターンはルーティングを終えたあとでないと決まらない。
			var route string
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				route = rctx.RoutePattern()
			}

			m.Observe(route, r.Method, sw.statusCode(), time.Since(start))
		})
	}
}
