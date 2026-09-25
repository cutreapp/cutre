// Package session はCookieに紐づくログイン中のセッションを扱う。
// リクエストのCookieから現在のユーザーを解決し、そのCookieの発行と削除を行う。
package session

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
)

// CookieName はセッショントークンを入れるCookieの名前。
//
// __Host- 接頭辞は、Secure属性を持ちPath=/でDomain属性を持たないCookieだけに許される。
// ブラウザはこの3つを満たさないCookieを拒むため、サブドメインから上書きされる経路が閉じる。
// Cutreが配信するのは本番 (Cloudflare) も開発 (exe.devのプロキシ) もHTTPSで、
// 手元のlocalhostもブラウザが安全なオリジンとして扱うため、Secureを常に付けても開発の妨げにならない。
const CookieName = "__Host-cutre_session"

// Manager はセッションCookieから現在のユーザーを解決し、そのCookieのライフサイクルを管理する。
//
// セッションの作成自体は行わない。セッション行の永続化はログインのUseCaseの責務であり、
// UseCaseが発行した平文のトークンをハンドラーが SetSessionCookie に渡す。
type Manager struct {
	userSessionRepo userSessionRepository
}

// userSessionRepository はセッションの解決と延長に必要な永続化操作。
// Managerが使う操作だけに絞ることで、データベース障害時の分岐をスタブで再現できるようにする。
type userSessionRepository interface {
	FindLiveWithUserByTokenDigest(ctx context.Context, tokenDigest string) (*model.UserSession, error)
	Extend(ctx context.Context, id model.UserSessionID, expiresAt, lastSeenAt time.Time) error
}

// NewManager は Manager を生成する。
func NewManager(userSessionRepo userSessionRepository) *Manager {
	return &Manager{userSessionRepo: userSessionRepo}
}

// GetCurrentUser はリクエストのセッションCookieをログイン中のユーザーに解決する。
// 未ログインのとき (Cookieが無い、トークンが未知・期限切れ、持ち主が退会済み) は (nil, nil) を返す。
// 非nilのエラーは本物の失敗 (データベースに到達できないなど) だけに使う。
//
// 解決したセッションは、必要なら有効期限を延長する。
// wを受け取るのはそのためで、サーバー側の期限とCookieの寿命を同時に進める。
func (m *Manager) GetCurrentUser(ctx context.Context, w http.ResponseWriter, r *http.Request) (*model.User, error) {
	token := m.SessionToken(r)
	if token == "" {
		return nil, nil
	}

	userSession, err := m.userSessionRepo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(token))
	if err != nil {
		return nil, err
	}
	if userSession == nil {
		return nil, nil
	}

	m.extendIfNeeded(ctx, w, token, userSession)

	return userSession.User, nil
}

// SessionToken はリクエストのCookieからセッショントークンを返す。Cookieが無いときは空文字列を返す。
// ログアウトは同じトークンでセッション行を消すため、トークンの読み取りはここに集約する。
func (m *Manager) SessionToken(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// SetSessionCookie はセッショントークンをCookieに書き込む。
func (m *Manager) SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, sessionCookie(token, int(model.UserSessionLifetime.Seconds())))
}

// DeleteSessionCookie はMaxAgeを負にした同名のCookieを送り、ブラウザにセッションCookieの削除を指示する。
// 他の属性を SetSessionCookie と揃えるのは、ブラウザが一致するCookieだけを削除するため。
func (m *Manager) DeleteSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, sessionCookie("", -1))
}

// extendIfNeeded は有効期限を延長すべきセッションの期限を進め、Cookieの寿命も同じだけ進める。
//
// 延長に失敗してもリクエストは続ける。期限はまだ残っており、
// 期限を伸ばせなかったことで今のリクエストの応答を止める理由にはならないため。
func (m *Manager) extendIfNeeded(ctx context.Context, w http.ResponseWriter, token string, userSession *model.UserSession) {
	now := time.Now()
	if !userSession.NeedsExtension(now) {
		return
	}

	if err := m.userSessionRepo.Extend(ctx, userSession.ID, model.UserSessionExpiresAt(now), now); err != nil {
		slog.WarnContext(ctx, "セッションの有効期限の延長に失敗しました", "error", err)
		return
	}

	m.SetSessionCookie(w, token)
}

// sessionCookie はセッションCookieを組み立てる。
//
// Secureは常に付ける。__Host- 接頭辞を持つCookieの条件であり、配信はどの環境でもHTTPSのためである。
// HttpOnlyでJavaScriptから読めないようにし、SameSite=Laxでクロスサイトの送出を制限する。
// Domainを設定しないのも接頭辞の条件で、Cookieはこのホストだけに送られる。
func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}
