package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

type failingUserSessionRepository struct {
	err error
}

func (r *failingUserSessionRepository) FindLiveWithUserByTokenDigest(context.Context, string) (*model.UserSession, error) {
	return nil, r.err
}

func (r *failingUserSessionRepository) Extend(context.Context, model.UserSessionID, time.Time, time.Time) error {
	return nil
}

// userReporter は現在のユーザーを本文に書き出すハンドラー。
// ミドルウェアがcontextに載せた結果を、テストからレスポンス越しに観測するために使う。
func userReporter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserFromContext(r.Context())
		if user == nil {
			_, _ = w.Write([]byte("anonymous"))
			return
		}

		_, _ = w.Write([]byte(user.Atname))
	})
}

// TestAuth_SetUser は、セッションCookieの状態に応じてcontextに載る結果が変わり、
// いずれの場合もリクエストがハンドラーまで届くことを検証する。
func TestAuth_SetUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	auth := middleware.NewAuth(session.NewManager(repository.NewUserSessionRepository(db).WithTx(tx)))

	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, tx).WithAtname(atname).Build()
	signedIn := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	signedIn.Build()

	expired := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build()).
		WithExpiresAt(time.Now().Add(-time.Minute))
	expired.Build()

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "ログイン中のセッション", token: signedIn.Token(), want: atname},
		{name: "Cookieが無い", token: "", want: "anonymous"},
		{name: "未知のトークン", token: "unknown-token", want: "anonymous"},
		{name: "期限切れのセッション", token: expired.Token(), want: "anonymous"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			auth.SetUser(userReporter()).ServeHTTP(rec, requestWithToken(tt.token))

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Body.String(); got != tt.want {
				t.Errorf("本文 = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestAuth_SessionLookupError は、セッション検索の失敗を未ログインと混同せず、
// ルートの公開範囲に応じた応答へ分けることを検証する。
func TestAuth_SessionLookupError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("セッション検索の失敗")
	auth := middleware.NewAuth(session.NewManager(&failingUserSessionRepository{err: wantErr}))

	tests := []struct {
		name       string
		middleware func(http.Handler) http.Handler
		wantStatus int
		wantBody   string
	}{
		{
			name:       "SetUserは匿名のまま後続へ渡す",
			middleware: auth.SetUser,
			wantStatus: http.StatusOK,
			wantBody:   "anonymous",
		},
		{
			name: "RequireAuthは500で止める",
			middleware: func(next http.Handler) http.Handler {
				return auth.SetUser(auth.RequireAuth(next))
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Internal Server Error\n",
		},
		{
			name: "RequireNoAuthは匿名のまま後続へ渡す",
			middleware: func(next http.Handler) http.Handler {
				return auth.SetUser(auth.RequireNoAuth(next))
			},
			wantStatus: http.StatusOK,
			wantBody:   "anonymous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			tt.middleware(userReporter()).ServeHTTP(rec, requestWithToken("token"))

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("本文 = %q、期待値 = %q", got, tt.wantBody)
			}
		})
	}
}

// TestAuth_RequireAuth_SignedIn は、ログイン中のリクエストがハンドラーへ届くことを検証する。
func TestAuth_RequireAuth_SignedIn(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	auth := middleware.NewAuth(session.NewManager(repository.NewUserSessionRepository(db).WithTx(tx)))

	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, tx).WithAtname(atname).Build()
	signedIn := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	signedIn.Build()

	rec := httptest.NewRecorder()
	auth.SetUser(auth.RequireAuth(userReporter())).ServeHTTP(rec, requestWithToken(signedIn.Token()))

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != atname {
		t.Errorf("本文 = %q、期待値 = %q", got, atname)
	}
}

// TestAuth_RequireAuth_Anonymous は、未ログインのリクエストがハンドラーに到達せず、
// 戻り先を持ってログイン画面へ送られることを検証する。
func TestAuth_RequireAuth_Anonymous(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	auth := middleware.NewAuth(session.NewManager(repository.NewUserSessionRepository(db).WithTx(tx)))

	tests := []struct {
		name     string
		method   string
		target   string
		wantPath string
	}{
		{
			name:     "GETは戻り先を載せる",
			method:   http.MethodGet,
			target:   "/settings/invitations?page=2",
			wantPath: "/sign_in?return_to=%2Fsettings%2Finvitations%3Fpage%3D2",
		},
		{
			name:     "HEADも戻り先を載せる",
			method:   http.MethodHead,
			target:   "/settings/invitations",
			wantPath: "/sign_in?return_to=%2Fsettings%2Finvitations",
		},
		{
			name:     "POSTは素のログイン画面へ送る",
			method:   http.MethodPost,
			target:   "/settings/invitations",
			wantPath: "/sign_in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false
			handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				handlerCalled = true
			})

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.target, nil)
			auth.SetUser(auth.RequireAuth(handler)).ServeHTTP(rec, req)

			if handlerCalled {
				t.Error("未ログインのリクエストがハンドラーに到達した")
			}
			if rec.Code != http.StatusSeeOther {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
			}
			if got := rec.Header().Get("Location"); got != tt.wantPath {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantPath)
			}
		})
	}
}

// TestAuth_RequireAuth_WithoutSetUser は、SetUser を通していないリクエストを
// ログイン画面へ送らずに500で止めることを検証する。
// 解決結果が無いのは配線の誤りであり、未ログインとして扱うと原因が見えなくなる。
func TestAuth_RequireAuth_WithoutSetUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	auth := middleware.NewAuth(session.NewManager(repository.NewUserSessionRepository(db).WithTx(tx)))

	handlerCalled := false
	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
	})

	rec := httptest.NewRecorder()
	auth.RequireAuth(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/home", nil))

	if handlerCalled {
		t.Error("解決結果の無いリクエストがハンドラーに到達した")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestAuth_RequireNoAuth_WithoutSetUser は、SetUser を通していないリクエストを
// ページを見せずに500で止めることを検証する。
// 解決結果が無いのは配線の誤りであり、未ログインとして扱うとログイン済みの訪問者に
// ログイン画面を見せ続ける誤りが静かに残る。
func TestAuth_RequireNoAuth_WithoutSetUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	auth := middleware.NewAuth(session.NewManager(repository.NewUserSessionRepository(db).WithTx(tx)))

	handlerCalled := false
	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
	})

	rec := httptest.NewRecorder()
	auth.RequireNoAuth(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sign_in", nil))

	if handlerCalled {
		t.Error("解決結果の無いリクエストがハンドラーに到達した")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestAuth_RequireNoAuth は、ログイン中の訪問者だけがホームへ送られることを検証する。
func TestAuth_RequireNoAuth(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	auth := middleware.NewAuth(session.NewManager(repository.NewUserSessionRepository(db).WithTx(tx)))

	signedIn := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build())
	signedIn.Build()

	tests := []struct {
		name         string
		token        string
		wantStatus   int
		wantLocation string
	}{
		{name: "ログイン中はホームへ送る", token: signedIn.Token(), wantStatus: http.StatusSeeOther, wantLocation: "/home"},
		{name: "未ログインはページを見せる", token: "", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			auth.SetUser(auth.RequireNoAuth(userReporter())).ServeHTTP(rec, requestWithToken(tt.token))

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
		})
	}
}

// TestSetUserToContext は、ハンドラーのテストが認証のミドルウェアを通さずに
// ログイン中の状態を作れることを検証する。
func TestSetUserToContext(t *testing.T) {
	t.Parallel()

	user := &model.User{Atname: "cutre"}

	ctx := middleware.SetUserToContext(context.Background(), user)

	got := middleware.UserFromContext(ctx)
	if got == nil {
		t.Fatal("ユーザー = nil、非nilを期待")
	}
	if got.Atname != user.Atname {
		t.Errorf("Atname = %q、期待値 = %q", got.Atname, user.Atname)
	}

	if middleware.UserFromContext(context.Background()) != nil {
		t.Error("ユーザーを載せていないcontextから非nilが返った")
	}
}

// requestWithToken はセッションCookieを載せたリクエストを返す。tokenが空ならCookieを付けない。
func requestWithToken(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: token})
	}

	return req
}
