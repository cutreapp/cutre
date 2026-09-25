package middleware

import "net/http"

// noStoreCacheControl はブラウザにも共有キャッシュにも応答を保存させないときのCache-Controlの値。
//
// no-storeはHTTPキャッシュへの保存を禁じる。BFCacheからの除外は保証しないため、
// 認証フォームの入力内容はBFCacheからの復元時に別途消す。
const noStoreCacheControl = "private, no-store"

// NoStore は応答をHTTPキャッシュに保存させない。
//
// 認証まわりの画面 (資格情報を入力するフォーム) とログイン後のページのルートグループに掛ける。
// 認証のミドルウェアより外側に置き、ログイン状態によって返すリダイレクトにも同じ方針を載せる。
// 同じURLが訪問者によってページにもリダイレクトにもなるため、どちらも保存させない。
//
// middleware.HTMLCache は応答を書き出す時点で方針の有無を見るため、ここで入れた値を既定値で上書きしない。
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", noStoreCacheControl)

		next.ServeHTTP(w, r)
	})
}
