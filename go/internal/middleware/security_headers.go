package middleware

import "net/http"

// securityHeaders は全レスポンスに付ける汎用のセキュリティヘッダー。
//
// いずれも配信するコンテンツを調べなくても値が決まるものに絞っている。
// CSP本体 (script-srcなど) は許可するオリジンの観測が要り、HSTSはTLSを終端する層と
// 足並みを揃える必要があるため、固定値を配る本ミドルウェアでは扱わない。
//
// frame-ancestorsはCSPのうち固定値で決まるためここで送る。
// CSP本体を導入するときは本体のポリシーへ統合する (1つのレスポンスに複数のCSPが載ると、
// ブラウザはそれぞれを独立に適用するため、本体側にframe-ancestorsを書き漏らしても緩まない)。
// X-Frame-Optionsは、frame-ancestorsを解釈しない古いブラウザ向けのフォールバックとして併送する。
var securityHeaders = map[string]string{
	"Referrer-Policy":         "strict-origin-when-cross-origin",
	"X-Content-Type-Options":  "nosniff",
	"Content-Security-Policy": "frame-ancestors 'none'",
	"X-Frame-Options":         "DENY",
	"Permissions-Policy":      "camera=(), microphone=(), geolocation=(), payment=()",
}

// SecurityHeaders は全レスポンスにsecurityHeadersを付ける。
//
// ミドルウェアチェーンの先頭に登録する。ヘッダーを書き込んでから後続へ渡すため、
// 内側で書き出されるレスポンス (chiのRecovererがpanicから返す500や、静的アセットの配信) にも載る。
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		for name, value := range securityHeaders {
			header.Set(name, value)
		}

		next.ServeHTTP(w, r)
	})
}
