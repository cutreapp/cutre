package profile_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/profile"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newRequest は、ルーターがパスから取り出したアットネームを持つリクエストを作る。
func newRequest(ctx context.Context, atname string, user *model.User) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/@"+atname, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("atname", atname)
	ctx = context.WithValue(i18n.SetLocale(ctx, i18n.LangEn), chi.RouteCtxKey, routeCtx)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}
	return req.WithContext(ctx)
}

// newHandler はテスト用のトランザクションの中で動く Handler を返す。
func newHandler(t *testing.T) (*profile.Handler, *sql.Tx) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	getInvitationRedemptionsUC := usecase.NewGetInvitationRedemptionsUsecase(repository.NewInvitationRedemptionRepository(db).WithTx(tx))
	getTwoFactorAuthStatusUC := usecase.NewGetTwoFactorAuthStatusUsecase(
		repository.NewUserTwoFactorAuthRepository(db).WithTx(tx),
		repository.NewUserTwoFactorRecoveryCodeRepository(db).WithTx(tx),
	)

	return profile.NewHandler(cfg, httperror.NewRenderer(cfg), getInvitationRedemptionsUC, getTwoFactorAuthStatusUC), tx
}

// TestShow は、自分のプロフィールが招待の画面への入口 (残りの人数と参加した人数)・二要素認証の画面への入口 (オフのバッジ付き)・
// ログアウトのフォーム (DELETEに上書きしたPOST) を持ち、
// 言語版を持たないページとしてcanonicalも別言語版への参照も宣言せず、インデックスも断ることを検証する。
// アットネームは大文字小文字を区別しないため、綴りの大小が違っても自分のプロフィールとして描く。
func TestShow(t *testing.T) {
	t.Parallel()

	for _, atname := range []string{"cutre_user", "Cutre_User"} {
		t.Run(atname, func(t *testing.T) {
			t.Parallel()

			const csrfToken = "profile-csrf-token"
			handler, tx := newHandler(t)
			userID := testutil.NewUserBuilder(t, tx).Build()
			invitationID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(userID).Build()
			testutil.NewInvitationRedemptionBuilder(t, tx, invitationID).Build()
			testutil.NewInvitationRedemptionBuilder(t, tx, invitationID).Build()

			req := newRequest(t.Context(), atname, &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn})
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
			rec := httptest.NewRecorder()
			middleware.NewCSRF().Middleware(http.HandlerFunc(handler.Show)).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			for _, want := range []string{
				`<html lang="en">`,
				"<title>Profile | Cutre</title>",
				`<h1 class="px-4 text-2xl font-bold tracking-tight md:px-0">Profile</h1>`,
				`href="/@cutre_user" class="btn aria-[current=page]:bg-accent" data-variant="ghost" data-size="sm" aria-current="page"`,
				`action="/user_session" method="post"`,
				`name="_method" value="DELETE"`,
				`name="csrf_token" value="` + csrfToken + `"`,
				"Sign out",
				`href="/settings/invitation"`,
				"Invitations",
				"Invites left: 3 · Joined: 2",
				`href="/settings/two_factor_auth"`,
				"Two-factor authentication",
				`<span class="badge" data-variant="outline">Off</span>`,
				`href="/settings/withdrawal"`,
				"Delete account",
				`<meta name="robots" content="noindex">`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
			for _, absent := range []string{`rel="canonical"`, `rel="alternate"`, `rel="preconnect"`} {
				if strings.Contains(body, absent) {
					t.Errorf("レスポンスボディに %q が含まれている", absent)
				}
			}
		})
	}
}

// TestShow_TwoFactorAuthEnabled は、二要素認証を有効にしていれば、二要素認証の入口にオンのバッジを付けることを検証する。
// 認証アプリへの登録の途中の設定では、まだオフとして描く。
func TestShow_TwoFactorAuthEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		enabled   bool
		wantBadge string
	}{
		{name: "有効", enabled: true, wantBadge: `<span class="badge" data-variant="success">On</span>`},
		{name: "登録の途中", enabled: false, wantBadge: `<span class="badge" data-variant="outline">Off</span>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, tx := newHandler(t)
			userID := testutil.NewUserBuilder(t, tx).Build()
			builder := testutil.NewUserTwoFactorAuthBuilder(t, tx, userID)
			if tt.enabled {
				builder.WithEnabledAt(time.Now())
			}
			builder.Build()

			rec := httptest.NewRecorder()
			handler.Show(rec, newRequest(t.Context(), "cutre_user", &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn}))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBadge) {
				t.Errorf("レスポンスボディに %q が含まれていない", tt.wantBadge)
			}
		})
	}
}

// TestShow_OtherAtname は、他の人のプロフィールを描けるようになるまで、自分以外のアットネームを404にすることを検証する。
func TestShow_OtherAtname(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t.Context(), "someone_else", &model.User{Atname: "cutre_user", Locale: model.LocaleEn}))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if strings.Contains(rec.Body.String(), "Sign out") {
		t.Error("404のページにログアウトのフォームが含まれている")
	}
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かのプロフィールとして描画しないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t.Context(), "cutre_user", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
