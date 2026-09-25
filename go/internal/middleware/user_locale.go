package middleware

import (
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
)

// UserLocale はログイン後のページの表示言語を、ログイン中のユーザーの users.locale に切り替える。
// RequireAuth の内側に掛け、ログインを求めるルートだけに適用する。
//
// I18n がURLから決めたロケールを上書きする形にするのは、URLで言語版を表すのが公開ページだけのため。
// ログイン後のページは言語版のURLを持たず、どのURLで開いても利用者が選んだ言語で表示する。
// 全ルートの I18n で切り替えると、ログイン中に開いた公開ページまでURLと違う言語で表示され、
// そのページが宣言する言語版の対応 (hreflang) と中身が食い違う。
//
// 別の言語版の案内も消す。案内の行き先となる言語版のURLが、ログイン後のページには無いため。
func UserLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := i18n.SetLocale(r.Context(), string(user.Locale))
		ctx = i18n.SetSuggestedLocale(ctx, "")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
