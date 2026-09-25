package middleware

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/auth"
)

// CSRFCookieName はCSRFトークンを入れるCookieの名前。
// セッションCookieと同じく __Host- 接頭辞を付け、サブドメインから上書きされる経路を閉じる。
const CSRFCookieName = "__Host-cutre_csrf"

// CSRFFieldName はフォームがCSRFトークンを載せるフィールドの名前。
const CSRFFieldName = "csrf_token"

// CSRFHeaderName はフォーム以外の送信 (htmxなど) がCSRFトークンを載せるヘッダーの名前。
const CSRFHeaderName = "X-CSRF-Token"

// csrfCookieMaxAge はCSRF Cookieの有効期間 (秒)。
// 期限が切れても次の安全なリクエストで新しいトークンを発行するため、
// これから送信するフォームには常に照合できるCookieがある。
const csrfCookieMaxAge = 24 * 60 * 60

// csrfTokenContextKey はCSRFトークンをリクエストcontextに載せるときのキー。
const csrfTokenContextKey contextKey = "csrfToken"

// CSRF は状態を変えるリクエストをCSRFから保護する。
//
// 保護は2段にする。
// 1段目はGoの net/http が持つオリジンの検査で、Sec-Fetch-SiteヘッダーまたはOriginヘッダーから
// クロスサイトのリクエストを見分けて弾く。
// 2段目はダブルサブミットCookieで、Cookieのトークンと送信されたトークンの一致を求める。
//
// 1段目だけにしないのは、ヘッダーを送らない経路 (古いブラウザや非ブラウザのクライアント) を
// net/http が同一オリジンとみなして通すため。
// 2段目だけにしないのは、Cookieを読めない攻撃者でもCookieの値を推測せずに済む経路 (サブドメインからの
// Cookie上書きなど) を、オリジンの検査が独立して塞ぐため。
//
// セッションに紐づくトークンではなくダブルサブミットCookieを使うのは、保護が最も要るフォーム
// (ログイン・登録) をセッションを持たない訪問者が送信するためである。
type CSRF struct {
	crossOrigin *http.CrossOriginProtection
}

// NewCSRF は CSRF を生成する。
func NewCSRF() *CSRF {
	return &CSRF{crossOrigin: http.NewCrossOriginProtection()}
}

// Middleware は安全なリクエストでCSRFトークンを発行し、それ以外のリクエストで検証する。
//
// 安全なメソッド (GET / HEAD / OPTIONS) は副作用を持たないため、トークンを発行または再利用し、
// テンプレートがフォームへ埋め込めるようcontextに載せて素通しする。
// それ以外のメソッドは検証に通らなければハンドラーに到達させず、403で応える。
//
// 応答が共通のエラーページではなく素のテキストなのは、ミドルウェアがエラーページの描画
// (internal/httperror) に依存できないため。
func (c *CSRF) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			token, err := c.issueToken(w, r)
			if err != nil {
				// 乱数の生成に失敗したときだけここに来る。まれな失敗でも調査の手がかりを残す。
				slog.ErrorContext(r.Context(), "CSRFトークンの生成に失敗しました", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfTokenContextKey, token)))
			return
		}

		if err := c.crossOrigin.Check(r); err != nil {
			slog.WarnContext(r.Context(), "クロスサイトのリクエストを拒否しました", "error", err)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		cookie, err := r.Cookie(CSRFCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		if !csrfTokenMatches(requestCSRFToken(r), cookie.Value) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// 検証済みのトークンをcontextに載せる。
		// バリデーションエラーでフォームを描き直すハンドラーが、同じトークンをフォームへ戻せるようにする。
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfTokenContextKey, cookie.Value)))
	})
}

// CSRFTokenFromContext はctxに載っているCSRFトークンを返す。
// 載っていないとき (本ミドルウェアを通っていないとき) は空文字列を返す。
// ハンドラーがこれを読み、フォームのhiddenフィールドへ渡す。
func CSRFTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(csrfTokenContextKey).(string)

	return token
}

// issueToken はリクエストのCookieにあるトークンを返し、無ければ新しく発行してCookieに書き込む。
func (c *CSRF) issueToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(CSRFCookieName); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	token, err := auth.GenerateSecureToken()
	if err != nil {
		return "", err
	}

	// HttpOnlyを付けるのは、トークンをフォームとヘッダーへ載せるのがサーバー側の描画だけで足り、
	// JavaScriptがCookieを読む必要がないため。読めないようにしておくと、XSSが起きたときに
	// トークンを持ち出す手段が1つ減る。
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   csrfCookieMaxAge,
	})

	return token, nil
}

// requestCSRFToken はリクエストが提示したCSRFトークンを返す。
// フォームのフィールドを先に見て、無ければヘッダーへ下がる。
func requestCSRFToken(r *http.Request) string {
	if token := r.PostFormValue(CSRFFieldName); token != "" {
		return token
	}

	return r.Header.Get(CSRFHeaderName)
}

// csrfTokenMatches は2つのトークンが一致するかを定数時間で比較して返す。
//
// 比較に掛かる時間が一致した先頭の長さで変わると、そこから1文字ずつ正解を絞り込めてしまう。
// 空のトークンは常に不一致として扱う。Cookieが空のときに空の提示で通ることを防ぐ。
func csrfTokenMatches(requestToken, cookieToken string) bool {
	if requestToken == "" || cookieToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(requestToken), []byte(cookieToken)) == 1
}

// isSafeMethod は副作用を持たないメソッドかどうかを返す。
func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
