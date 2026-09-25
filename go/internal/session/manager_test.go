package session_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

type stubUserSessionRepository struct {
	userSession *model.UserSession
	findErr     error
	extendErr   error
	extended    bool
}

func (r *stubUserSessionRepository) FindLiveWithUserByTokenDigest(context.Context, string) (*model.UserSession, error) {
	return r.userSession, r.findErr
}

func (r *stubUserSessionRepository) Extend(context.Context, model.UserSessionID, time.Time, time.Time) error {
	r.extended = true
	return r.extendErr
}

// requestWithToken はセッションCookieを載せたリクエストを返す。
func requestWithToken(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: token})

	return req
}

// TestManager_GetCurrentUser は、有効なセッションCookieが持ち主のユーザーに解決されることを検証する。
func TestManager_GetCurrentUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	mgr := session.NewManager(repository.NewUserSessionRepository(db).WithTx(tx))

	userID := testutil.NewUserBuilder(t, tx).Build()
	builder := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	builder.Build()

	rec := httptest.NewRecorder()
	user, err := mgr.GetCurrentUser(ctx, rec, requestWithToken(builder.Token()))
	if err != nil {
		t.Fatalf("GetCurrentUser()のエラー = %v", err)
	}

	if user == nil {
		t.Fatal("ユーザー = nil、非nilを期待")
	}
	if user.ID != userID {
		t.Errorf("ユーザーID = %s、期待値 = %s", user.ID, userID)
	}
}

// TestManager_GetCurrentUser_Anonymous は、ログインしていないリクエストが
// エラーではなくユーザー無しとして扱われることを検証する。
func TestManager_GetCurrentUser_Anonymous(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	mgr := session.NewManager(repository.NewUserSessionRepository(db).WithTx(tx))

	expired := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build()).
		WithExpiresAt(time.Now().Add(-time.Minute))
	expired.Build()

	tests := []struct {
		name string
		req  *http.Request
	}{
		{name: "Cookieが無い", req: httptest.NewRequest(http.MethodGet, "/", nil)},
		{name: "未知のトークン", req: requestWithToken("unknown-token")},
		{name: "期限切れのセッション", req: requestWithToken(expired.Token())},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			user, err := mgr.GetCurrentUser(ctx, rec, tt.req)
			if err != nil {
				t.Fatalf("GetCurrentUser()のエラー = %v", err)
			}

			if user != nil {
				t.Errorf("ユーザー = %v、期待値 = nil", user)
			}
		})
	}
}

// TestManager_GetCurrentUser_Extends は、しばらく使っていなかったセッションが
// アクセスによって延長され、Cookieの寿命も同時に進むことを検証する。
func TestManager_GetCurrentUser_Extends(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)
	mgr := session.NewManager(repo)

	builder := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build()).
		WithLastSeenAt(time.Now().Add(-48 * time.Hour)).
		WithExpiresAt(time.Now().Add(24 * time.Hour))
	builder.Build()

	rec := httptest.NewRecorder()
	if _, err := mgr.GetCurrentUser(ctx, rec, requestWithToken(builder.Token())); err != nil {
		t.Fatalf("GetCurrentUser()のエラー = %v", err)
	}

	extended, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(builder.Token()))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
	}
	if extended == nil {
		t.Fatal("セッション = nil、非nilを期待")
	}
	if !extended.ExpiresAt.After(time.Now().Add(24 * time.Hour)) {
		t.Errorf("ExpiresAt = %v、延長されていない", extended.ExpiresAt)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookieの数 = %d、期待値 = 1", len(cookies))
	}
	if cookies[0].Value != builder.Token() {
		t.Errorf("Cookieの値 = %q、期待値 = %q", cookies[0].Value, builder.Token())
	}
}

// TestManager_GetCurrentUser_ExtensionError は、期限延長だけに失敗した場合に現在のリクエストを
// 認証済みとして続け、サーバー側と期限が揃わないCookieを再発行しないことを検証する。
func TestManager_GetCurrentUser_ExtensionError(t *testing.T) {
	t.Parallel()

	wantUser := &model.User{Atname: "cutre"}
	repo := &stubUserSessionRepository{
		userSession: &model.UserSession{
			LastSeenAt: time.Now().Add(-48 * time.Hour),
			User:       wantUser,
		},
		extendErr: errors.New("セッション延長の失敗"),
	}
	mgr := session.NewManager(repo)
	rec := httptest.NewRecorder()

	gotUser, err := mgr.GetCurrentUser(context.Background(), rec, requestWithToken("token"))
	if err != nil {
		t.Fatalf("GetCurrentUser()のエラー = %v", err)
	}
	if gotUser != wantUser {
		t.Errorf("ユーザー = %v、期待値 = %v", gotUser, wantUser)
	}
	if !repo.extended {
		t.Error("セッションの延長が呼ばれていない")
	}
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Errorf("Set-Cookie = %q、期待値 = 空文字列", got)
	}
}

// TestManager_GetCurrentUser_SkipsExtension は、最近使ったセッションで
// 延長のUPDATEもCookieの再発行も起きないことを検証する。
// リクエストのたびに書き込むことを避けるのが、間引きの目的である。
func TestManager_GetCurrentUser_SkipsExtension(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)
	mgr := session.NewManager(repo)

	builder := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build()).
		WithLastSeenAt(time.Now())
	builder.Build()

	before, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(builder.Token()))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
	}
	if before == nil {
		t.Fatal("セッション = nil、非nilを期待")
	}

	rec := httptest.NewRecorder()
	if _, err := mgr.GetCurrentUser(ctx, rec, requestWithToken(builder.Token())); err != nil {
		t.Fatalf("GetCurrentUser()のエラー = %v", err)
	}

	after, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(builder.Token()))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
	}
	if after == nil {
		t.Fatal("セッション = nil、非nilを期待")
	}
	if !after.ExpiresAt.Equal(before.ExpiresAt) {
		t.Errorf("ExpiresAt = %v、期待値 = %v", after.ExpiresAt, before.ExpiresAt)
	}

	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("Set-Cookieの数 = %d、期待値 = 0", len(cookies))
	}
}

// TestManager_SetSessionCookie は、発行するCookieが __Host- 接頭辞の条件を満たし、
// JavaScriptとクロスサイトの送出から守られていることを検証する。
func TestManager_SetSessionCookie(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil)

	rec := httptest.NewRecorder()
	mgr.SetSessionCookie(rec, "token")

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookieの数 = %d、期待値 = 1", len(cookies))
	}
	cookie := cookies[0]

	if cookie.Name != session.CookieName {
		t.Errorf("Cookie名 = %q、期待値 = %q", cookie.Name, session.CookieName)
	}
	if cookie.Value != "token" {
		t.Errorf("Cookieの値 = %q、期待値 = %q", cookie.Value, "token")
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q、期待値 = %q", cookie.Path, "/")
	}
	if cookie.Domain != "" {
		t.Errorf("Domain = %q、期待値 = 空文字列", cookie.Domain)
	}
	if !cookie.Secure {
		t.Error("Secure = false、期待値 = true")
	}
	if !cookie.HttpOnly {
		t.Error("HttpOnly = false、期待値 = true")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v、期待値 = %v", cookie.SameSite, http.SameSiteLaxMode)
	}
	if cookie.MaxAge != int(model.UserSessionLifetime.Seconds()) {
		t.Errorf("MaxAge = %d、期待値 = %d", cookie.MaxAge, int(model.UserSessionLifetime.Seconds()))
	}
}

// TestManager_DeleteSessionCookie は、ブラウザにCookieの削除を指示することを検証する。
func TestManager_DeleteSessionCookie(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil)

	rec := httptest.NewRecorder()
	mgr.DeleteSessionCookie(rec)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookieの数 = %d、期待値 = 1", len(cookies))
	}
	cookie := cookies[0]

	if cookie.Name != session.CookieName {
		t.Errorf("Cookie名 = %q、期待値 = %q", cookie.Name, session.CookieName)
	}
	if cookie.Value != "" {
		t.Errorf("Cookieの値 = %q、期待値 = 空文字列", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("MaxAge = %d、期待値 = 負の値", cookie.MaxAge)
	}
}

// TestManager_SessionToken は、Cookieが無いリクエストで空文字列を返すことを検証する。
// ログアウトが「消すセッションが無い」ことをこの値で判断する。
func TestManager_SessionToken(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil)

	if got := mgr.SessionToken(requestWithToken("token")); got != "token" {
		t.Errorf("SessionToken() = %q、期待値 = %q", got, "token")
	}
	if got := mgr.SessionToken(httptest.NewRequest(http.MethodGet, "/", nil)); got != "" {
		t.Errorf("SessionToken() = %q、期待値 = 空文字列", got)
	}
}
