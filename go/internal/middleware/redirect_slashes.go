package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// RedirectSlashes は末尾スラッシュ付きのパスを、スラッシュ無しのURLへ恒久リダイレクトで転送する。
// ルートの / はそのまま後続へ渡す。ルーター全体の先頭側で使用する。
//
// パスとクエリを別々に組み立て、ファイル名などに含まれる #・?・% が
// 転送先では区切り文字や不正なエスケープとして解釈されることを防ぐ。
func RedirectSlashes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if len(path) <= 1 || !strings.HasSuffix(path, "/") {
			next.ServeHTTP(w, r)
			return
		}

		// バックスラッシュを区切りと解釈するブラウザでも、転送先が //host のような
		// 外部ホストへの参照にならないよう、先頭の区切りはスラッシュ1本にする。
		// 途中と末尾のバックスラッシュはファイル名に使える文字であり、そのまま残す。
		target := url.URL{
			Path:       "/" + strings.TrimRight(strings.TrimLeft(path, `/\`), "/"),
			RawQuery:   r.URL.RawQuery,
			ForceQuery: r.URL.ForceQuery,
		}
		http.Redirect(w, r, target.String(), redirectSlashesStatus(r.Method))
	})
}

// redirectSlashesStatus は正規化の転送に使うステータスコードを返す。
//
// GET / HEAD以外に308を選ぶのは、301を受けたクライアントがメソッドをGETへ書き換え、
// 送られたフォームの中身を落とすことがあるためである。
// ルートを足す側はこのミドルウェアを経ることを意識しないため、ここでメソッドを保つ。
func redirectSlashesStatus(method string) int {
	if method == http.MethodGet || method == http.MethodHead {
		return http.StatusMovedPermanently
	}

	return http.StatusPermanentRedirect
}
