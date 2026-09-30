package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/admin"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// show は user として管理画面の入口を開く。user がnilなら RequireAuth を通していないリクエストにする。
func show(t *testing.T, user *model.User) *httptest.ResponseRecorder {
	t.Helper()

	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	handler := admin.NewHandler(cfg, httperror.NewRenderer(cfg), usecase.NewGetAdminMenuUsecase())

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}
	rec := httptest.NewRecorder()
	handler.Show(rec, httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx))

	return rec
}

// newUserWithRole は role の役割のユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func newUserWithRole(t *testing.T, role model.UserRole) *model.User {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	id := testutil.NewUserBuilder(t, tx).WithRole(role).Build()
	user, err := repository.NewUserRepository(db).WithTx(tx).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// TestShow は、編集者と管理者に管理画面の入口を描画し、イベントと駅の管理へのリンクを出すことを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	for _, role := range []model.UserRole{model.UserRoleEditor, model.UserRoleAdmin} {
		user := newUserWithRole(t, role)
		rec := show(t, user)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: ステータスコード = %d、期待値 = %d", role, rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		for _, want := range []string{
			"<title>管理画面 | Cutre</title>",
			`<h1 class="text-xl font-semibold">管理画面</h1>`,
			`href="/@` + user.Atname + `"`,
			`href="/admin/events"`,
			`href="/admin/stations"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: レスポンスボディに %q が含まれていない", role, want)
			}
		}
	}
}

// TestShow_NotFound は、一般のユーザーには管理画面の存在を明かさず、404を返すことを検証する。
func TestShow_NotFound(t *testing.T) {
	t.Parallel()

	rec := show(t, newUserWithRole(t, model.UserRoleUser))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストを描画しないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	rec := show(t, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
