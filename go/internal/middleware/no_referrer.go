package middleware

import "net/http"

// NoReferrer はURLに秘密を含むページからの遷移で、参照元のURLを送らせない。
func NoReferrer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
