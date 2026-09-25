// Package templates はtemplテンプレートから呼び出すヘルパーを提供する。
package templates

import (
	"context"
	"net/url"

	"github.com/cutreapp/cutre/go/internal/i18n"
)

// T はctxのロケールでmessageIDを翻訳する。
// テンプレートがi18nではなくtemplatesパッケージに依存するようにするためのi18n.Tの薄いラッパー。
func T(ctx context.Context, messageID string, templateData ...map[string]any) string {
	return i18n.T(ctx, messageID, templateData...)
}

// Locale はctxのロケールを返す。html要素のlang属性に使う。
// lang属性が取るのはBCP 47の言語タグで、ロケールはその綴りをそのまま持つため値は加工せずマークアップへ入る。
func Locale(ctx context.Context) string {
	return i18n.GetLocale(ctx)
}

// RootPath はctxのロケールの言語版のトップページのパスを返す。
// 言語版ごとにトップページのアドレスが違うため、行き先をテンプレートに直書きすると
// 英語版から日本語版のトップページへ送ることになる。
func RootPath(ctx context.Context) string {
	return i18n.LocalePath(i18n.GetLocale(ctx), "/")
}

// ReturnToParam はログイン後の戻り先を運ぶクエリパラメータとフォームの値の名前。
// ログインを求めるミドルウェアが付け、ログイン画面のフォームが運び、ログインの処理が読むため、ここに1つだけ置く。
const ReturnToParam = "return_to"

// WithReturnTo はパスにログイン後の戻り先をクエリで付けて返す。戻り先が空ならパスをそのまま返す。
// ログインの手順の画面の間で戻り先を引き継ぐのに使う。returnTo には検証済みの値だけを渡す。
func WithReturnTo(path, returnTo string) string {
	if returnTo == "" {
		return path
	}

	return path + "?" + url.Values{ReturnToParam: {returnTo}}.Encode()
}

// SignInPath はログイン画面のパス (言語コードを含まない形)。
const SignInPath = "/sign_in"

// SignInTwoFactorPath は、ログインで認証アプリのコードを入力する画面のパス (言語コードを含まない形)。
// パスワードを確かめた二要素認証のユーザーを、ログイン画面の言語版のまま送る。
const SignInTwoFactorPath = "/sign_in/two_factor"

// SignInTwoFactorRecoveryPath は、ログインでリカバリーコードを入力する画面のパス (言語コードを含まない形)。
const SignInTwoFactorRecoveryPath = "/sign_in/two_factor/recovery"

// HelpContactURL は、WikinoにあるCutreのヘルプのお問い合わせのページのURL。
// 認証アプリもリカバリーコードも無くしてログインできない人を案内する。ページを用意するまでの仮の値。
const HelpContactURL = "https://wikino.app/s/cutre"

// HomePath はログイン後のホームのパス。
// ログイン後のページは表示言語を users.locale で決めるため、言語版を持たない。
const HomePath = "/home"

// UserSessionPath はログイン中のセッションのパス。DELETEでログアウトする。
const UserSessionPath = "/user_session"

// SignUpPath は登録 (メールアドレスの入力) の画面のパス (言語コードを含まない形)。
const SignUpPath = "/sign_up"

// EmailConfirmationPath は確認コードの入力画面のパス (言語コードを含まない形)。
const EmailConfirmationPath = "/email_confirmation"

// AccountPath はアカウントの作成 (アットネームとパスワードの設定) の画面のパス (言語コードを含まない形)。
const AccountPath = "/account"

// PasswordResetPath はパスワードリセットの申請の画面のパス (言語コードを含まない形)。
const PasswordResetPath = "/password_reset"

// PasswordResetSentPath はパスワードリセットの申請を受け付けた後の画面のパス (言語コードを含まない形)。
const PasswordResetSentPath = "/password_reset/sent"

// PasswordPath は新しいパスワードを設定する画面のパス (言語コードを含まない形)。
// パスワードリセットのメールのリンクが、トークンをクエリに付けてここを指す。
const PasswordPath = "/password"

// InvitationPath は招待リンクのパスを返す。
// QRコードを粗く保てるよう、パスを短くしている。
func InvitationPath(token string) string {
	return "/i/" + token
}

// ProfilePath はアットネームのユーザーのプロフィールのパスを返す。
// 先頭の @ で他のパスと区別するため、アットネームにルーティングと衝突する予約語を設けずに済む。
func ProfilePath(atname string) string {
	return "/@" + atname
}

// SettingsInvitationPath は招待の画面のパス。ログイン後のページのため言語版を持たない。
const SettingsInvitationPath = "/settings/invitation"

// SettingsTwoFactorAuthPath は二要素認証の画面のパス。POSTで有効にする。ログイン後のページのため言語版を持たない。
const SettingsTwoFactorAuthPath = "/settings/two_factor_auth"

// NewSettingsTwoFactorAuthPath は二要素認証を有効にする (認証アプリへ登録する) 画面のパス。
const NewSettingsTwoFactorAuthPath = "/settings/two_factor_auth/new"

// SettingsWithdrawalPath は退会の画面のパス。DELETEで退会する。ログイン後のページのため言語版を持たない。
const SettingsWithdrawalPath = "/settings/withdrawal"

// LocalePath は言語コードを含まないパスを、ctxのロケールの言語版のパスへ変換する。
// フォームの送信先のように、表示中の言語版に留まるべき行き先を組み立てるのに使う。
func LocalePath(ctx context.Context, path string) string {
	return i18n.LocalePath(i18n.GetLocale(ctx), path)
}
