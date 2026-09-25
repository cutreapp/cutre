package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/cutreapp/cutre/go/internal/templates"
)

// SanitizeReturnTo はrawが安全にリダイレクトできる行き先を指しているとき、正規化したパスを返す。
// それ以外では空文字列を返す。
//
// 受け付けるのは同一オリジンの相対パスだけで、先頭が "/" 1つ、その次が "/" でも "\" でもないものに限る。
// 戻り先はアプリの外 (クエリパラメータ、後にフォームの値) から渡ってくるため、
// 別のオリジンを指す値 ("//example.com"、"https://example.com") や、
// ブラウザが別のオリジンとして解釈する値 ("/\example.com") は破棄する。
// これがログインの導線をオープンリダイレクトにしないための要である。
//
// 受け付けたパスは再エンコードし、フラグメントを落として返す。
func SanitizeReturnTo(raw string) string {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, `/\`) {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return ""
	}

	return u.RequestURI()
}

// signInPathWithReturnTo は未ログインのリクエストrに対するログイン画面のパスを、
// ログイン後に元のURLへ戻れるようリクエスト先を載せて返す。
//
// 載せるのはGETとHEADのときだけとする。
// 安全でないメソッドの宛先を後からGETでなぞると、訪問者が求めていないページに着地させてしまうため。
func signInPathWithReturnTo(r *http.Request) string {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return templates.SignInPath
	}

	returnTo := SanitizeReturnTo(r.URL.RequestURI())
	if returnTo == "" {
		return templates.SignInPath
	}

	return templates.SignInPath + "?" + url.Values{templates.ReturnToParam: {returnTo}}.Encode()
}
