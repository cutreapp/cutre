package middleware

import (
	"net/http"

	"github.com/cutreapp/cutre/go/internal/logging"
)

// requestIDHeader はリクエストIDをクライアントへ返すレスポンスヘッダー。
const requestIDHeader = "X-Request-Id"

// RequestID はリクエストごとにIDを採番し、リクエストのcontextとレスポンスの X-Request-Id に載せる。
//
// 受信した X-Request-Id は使わない。前段がインターネットに面しており、
// 利用者が任意の値を付けて他のリクエストのログに紛れ込ませられるため。
//
// ヘッダーを書き込んでから後続へ渡すため、内側のpanicから返す500にも載る。
// アクセスログより外側に登録し、アクセスログにもIDが載るようにする。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := logging.NewRequestID()

		w.Header().Set(requestIDHeader, id)

		next.ServeHTTP(w, r.WithContext(logging.WithRequestID(r.Context(), id)))
	})
}
