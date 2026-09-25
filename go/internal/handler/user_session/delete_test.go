package user_session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/handler/user_session"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// TestDelete は、ログアウトがセッションの行とCookieを消し、利用者の言語のトップページへ送ることと、
// ログインしていないリクエストも失敗させずに同じ形で応えることを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		signedIn     bool
		locale       model.Locale
		wantLocation string
	}{
		{name: "日本語の利用者は日本語版のトップページへ", signedIn: true, locale: model.LocaleJa, wantLocation: "/"},
		{name: "英語の利用者は英語版のトップページへ", signedIn: true, locale: model.LocaleEn, wantLocation: "/en"},
		{name: "ログインしていなければURLの言語のトップページへ", signedIn: false, wantLocation: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			ctx := context.Background()
			repo := repository.NewUserSessionRepository(db).WithTx(tx)
			handler := user_session.NewHandler(session.NewManager(repo), session.NewFlashManager(), usecase.NewDeleteSessionUsecase(repo))

			req := httptest.NewRequest(http.MethodDelete, "/user_session", nil)
			reqCtx := i18n.SetLocale(req.Context(), i18n.LangJa)

			var token string
			if tt.signedIn {
				userID := testutil.NewUserBuilder(t, tx).WithLocale(tt.locale).Build()
				userSession := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
				userSession.Build()
				token = userSession.Token()

				req.AddCookie(&http.Cookie{Name: session.CookieName, Value: token})
				reqCtx = middleware.SetUserToContext(reqCtx, &model.User{ID: userID, Locale: tt.locale})
			}

			rec := httptest.NewRecorder()
			handler.Delete(rec, req.WithContext(reqCtx))

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}

			var sessionCleared, flashSet bool
			for _, c := range rec.Result().Cookies() {
				switch c.Name {
				case session.CookieName:
					sessionCleared = c.MaxAge < 0
				case session.FlashCookieName:
					flashSet = c.Value != ""
				}
			}
			if !sessionCleared {
				t.Error("セッションCookieの削除が指示されていない")
			}
			if !flashSet {
				t.Error("フラッシュメッセージが設定されていない")
			}

			if tt.signedIn {
				found, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(token))
				if err != nil || found != nil {
					t.Errorf("ログアウト後のセッション = (%v, %v)、(nil, nil)を期待", found, err)
				}
			}
		})
	}
}

// TestDelete_DeleteSessionError は、セッションの削除に失敗したときに成功扱いせず、
// Cookieとフラッシュメッセージを設定しないことを検証する。
func TestDelete_DeleteSessionError(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserSessionRepository(db).WithTx(tx)
	handler := user_session.NewHandler(session.NewManager(repo), session.NewFlashManager(), usecase.NewDeleteSessionUsecase(repo))

	userID := testutil.NewUserBuilder(t, tx).Build()
	userSession := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	userSession.Build()

	req := httptest.NewRequest(http.MethodDelete, "/user_session", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: userSession.Token()})
	reqCtx := i18n.SetLocale(req.Context(), i18n.LangJa)
	reqCtx = middleware.SetUserToContext(reqCtx, &model.User{ID: userID, Locale: model.LocaleJa})
	reqCtx, cancel := context.WithCancel(reqCtx)
	cancel()

	rec := httptest.NewRecorder()
	handler.Delete(rec, req.WithContext(reqCtx))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if location := rec.Header().Get("Location"); location != "" {
		t.Errorf("Location = %q、空文字列を期待", location)
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == session.CookieName || cookie.Name == session.FlashCookieName {
			t.Errorf("失敗時にCookie %qが設定されている", cookie.Name)
		}
	}

	found, err := repo.FindLiveWithUserByTokenDigest(context.Background(), auth.HashToken(userSession.Token()))
	if err != nil || found == nil {
		t.Errorf("削除失敗後のセッション = (%v, %v)、残ることを期待", found, err)
	}
}
