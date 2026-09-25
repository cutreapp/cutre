package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/templates"
)

// contextKey はcontextのキーの型。他のパッケージが定義するキーと衝突しないよう非公開の型にする。
type contextKey string

// authResultContextKey は現在のユーザーの解決結果をリクエストcontextに載せるときのキー。
const authResultContextKey contextKey = "authResult"

// authResult は現在のユーザーの解決結果。
//
// 「ユーザーがいない」と「解決に失敗した」を区別して運ぶ。
// SetUser はどちらでもリクエストを通すが、ログインを要求するルートはこの2つで応答を変える。
type authResult struct {
	user *model.User
	err  error
}

// Auth は認証のミドルウェアの依存を保持する。
type Auth struct {
	sessionMgr *session.Manager
}

// NewAuth は Auth を生成する。
func NewAuth(sessionMgr *session.Manager) *Auth {
	return &Auth{sessionMgr: sessionMgr}
}

// SetUser はリクエストのセッションCookieから現在のユーザーを解決し、解決結果をリクエストcontextに載せる。
//
// リクエストを止めることはしない。未ログインのリクエストはユーザーの無いまま進み、
// 本物の解決失敗 (データベースに到達できないなど) もログに記録したうえで先へ進める。
// 一時的なデータベースの不調が、未ログインの訪問者でも見られるページを巻き込んで落とさないようにするためである。
// ログインの要求は関心事が別で、ルート単位の RequireAuth が担う。
//
// 全ルートに掛けるのは、表示言語の解決 (I18n) より先にユーザーを知る必要があるため。
// I18nより外側に置ける位置は全ルートの共通部分しかない。
func (a *Auth) SetUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		user, err := a.sessionMgr.GetCurrentUser(ctx, w, r)
		if err != nil {
			slog.WarnContext(ctx, "現在のユーザーの解決に失敗しました", "error", err)
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, authResultContextKey, authResult{user: user, err: err})))
	})
}

// RequireAuth はログインを要求するルートを保護する。
//
// 未ログインのGET / HEADのリクエストはハンドラーに到達させず、ログイン後に元のURLへ戻れるよう
// リクエスト先を return_to に載せてログイン画面へ送る。
// 他のメソッドは、宛先をログイン後にGETでなぞると求めていないページに着地させるため、素のログイン画面へ送る。
//
// SetUser と違い、ユーザーの解決に失敗したリクエストは500で応える。
// 訪問者が誰かわからないまま、その人向けのページを描画することはできないためである。
// 解決結果そのものが無いのは SetUser を通していない配線の誤りであり、同じく500で応える。
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, ok := authResultFromContext(r.Context())
		if !ok {
			slog.ErrorContext(r.Context(), "現在のユーザーの解決結果がありません (SetUser を通していません)")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if result.err != nil {
			slog.ErrorContext(r.Context(), "認証チェックに失敗しました", "error", result.err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if result.user == nil {
			http.Redirect(w, r, signInPathWithReturnTo(r), http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireNoAuth は未ログインの訪問者だけに見せるルート (ログイン・登録・パスワードリセット) を保護する。
// ログイン済みの訪問者はログイン後のホームへ送る。
//
// 解決に失敗したリクエストは未ログインとして扱い、ページを描画する。
// ここでのユーザーの有無は行き先を変えるだけで、見せてよい相手を狭めるものではないため、
// データベースの不調でログイン画面に入れなくなるほうが不都合が大きい。
//
// 解決結果そのものが無いのは SetUser を通していない配線の誤りであり、RequireAuth と同じく500で応える。
// 未ログインとして通すと、ログイン済みの訪問者にログイン画面を見せ続ける誤りが静かに残るためである。
func (a *Auth) RequireNoAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, ok := authResultFromContext(r.Context())
		if !ok {
			slog.ErrorContext(r.Context(), "現在のユーザーの解決結果がありません (SetUser を通していません)")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if result.user != nil {
			http.Redirect(w, r, templates.HomePath, http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// UserFromContext はctxに載っている現在のユーザーを返す。
// 未ログインのとき (および SetUser が走っていないとき) はnilを返す。
func UserFromContext(ctx context.Context) *model.User {
	result, ok := authResultFromContext(ctx)
	if !ok {
		return nil
	}

	return result.user
}

// SetUserToContext はuserを現在のユーザーとして載せたctxのコピーを返す。
// SetUser と同じキーに格納するため UserFromContext が読み戻せる。
// 主にハンドラーのテストが、リクエストを認証のミドルウェアに通さずにログイン中の状態を作るために使う。
func SetUserToContext(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, authResultContextKey, authResult{user: user})
}

// authResultFromContext はctxに載っている解決結果を、載っているかどうかとともに返す。
func authResultFromContext(ctx context.Context) (authResult, bool) {
	result, ok := ctx.Value(authResultContextKey).(authResult)

	return result, ok
}
