// Package middleware はアプリケーション固有のHTTPミドルウェアを提供する。
package middleware

import (
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
)

// SelectedLocaleCookie は利用者が明示的に選んだ言語版のロケールを記録するCookieの名前。
// 言語スイッチャーのリンク、案内の移動リンク、案内の閉じるボタンがこの名前で書き込む (web/locale-choice.js)。
//
// 閉じる操作も「表示中の言語版を選んだ」ものとして同じCookieに記録する。
// 案内を出すかどうかは選んだロケールと表示中のロケールの一致で決まるため、
// 閉じたことを別に覚える必要がない。
const SelectedLocaleCookie = "selected_locale"

// I18n はリクエストのロケールと、別の言語版として案内すべきロケールを解決し、
// 後続のハンドラーとテンプレートが参照できるようcontextに載せる。
//
// 表示するロケールは開いたURLだけで決める。利用者が開いたURLの内容をそのまま返すため、
// Accept-Languageや選択のCookieで表示言語を切り替えたり、そちらの言語版へ自動でリダイレクトしたりはしない。
// これらを使うのは、別の言語版があることを知らせる案内を出すかどうかの判定だけである。
//
// 本ミドルウェアが internal/i18n ではなくここにあるのは、将来ロケールの解決にログイン中のユーザーが
// 要るようになったときに、internal/i18n が本パッケージへ依存する形にならないようにするため。
func I18n(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := i18n.LocaleFromPath(r.URL.Path)

		ctx := i18n.SetLocale(r.Context(), locale)
		ctx = i18n.SetSuggestedLocale(ctx, suggestedLocale(r, locale))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// suggestedLocale は案内すべき言語版のロケールを返す。案内しないときは空文字列を返す。
//
// 判定をここに集めることで、案内を組み立てる側は結果のロケールだけを見れば済む。
func suggestedLocale(r *http.Request, currentLocale string) string {
	// 明示的に選んだ言語はブラウザの言語設定より優先する。
	// 推定を優先すると、スイッチャーで相手言語版へ移った利用者に元の言語版への案内が出続ける。
	wanted := selectedLocale(r)
	if wanted == "" {
		// 求められた言語に翻訳が無いときはDetectLanguageが空文字列を返し、そのまま案内しない扱いになる。
		wanted = i18n.DetectLanguage(r)
	}

	// 表示中の言語版が既に求められている言語なら案内するものが無い。
	if wanted == currentLocale {
		return ""
	}

	return wanted
}

// selectedLocale は利用者が明示的に選んだ言語版のロケールを返す。
// 選択が無いときと、翻訳を持たないロケールが書かれていたときは空文字列を返す。
//
// Cookieの値は利用者が書き換えられるため、翻訳を持つロケールかどうかをここで確かめる。
// 未対応の値を選択として扱うとブラウザの言語設定を参照せず、後段の正規化によって案内が出なくなる。
// そのため選択が無いものとして扱い、ブラウザの言語設定からの推定に戻す。
func selectedLocale(r *http.Request) string {
	cookie, err := r.Cookie(SelectedLocaleCookie)
	if err != nil || !i18n.IsSupportedLang(cookie.Value) {
		return ""
	}

	return cookie.Value
}
