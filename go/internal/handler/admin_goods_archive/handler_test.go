package admin_goods_archive_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/admin_goods_archive"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// fixture はテスト用のトランザクションの中で動く Handler と、そのトランザクションとグッズのリポジトリ。
type fixture struct {
	handler   *admin_goods_archive.Handler
	tx        *sql.Tx
	goodsRepo *repository.GoodsRepository
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	eventRepo := repository.NewEventRepository(db).WithTx(tx)
	categoryRepo := repository.NewEventCategoryRepository(db).WithTx(tx)
	goodsRepo := repository.NewGoodsRepository(db).WithTx(tx)
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	return &fixture{
		handler: admin_goods_archive.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetAdminGoodsUsecase(eventRepo, categoryRepo, goodsRepo),
			usecase.NewArchiveGoodsUsecase(validator.NewGoodsArchiveCreateValidator(), eventRepo, categoryRepo, goodsRepo),
			usecase.NewUnarchiveGoodsUsecase(eventRepo, categoryRepo, goodsRepo),
		),
		tx:        tx,
		goodsRepo: goodsRepo,
	}
}

// user は role の役割のユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func (f *fixture) user(t *testing.T, role model.UserRole) *model.User {
	t.Helper()

	id := testutil.NewUserBuilder(t, f.tx).WithRole(role).Build()
	user, err := repository.NewUserRepository(testutil.GetTestDB()).WithTx(f.tx).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// goods はテスト用のイベントとグッズの配下にグッズを作る GoodsBuilder を返す。
func (f *fixture) goods(t *testing.T) *testutil.GoodsBuilder {
	t.Helper()

	return testutil.NewGoodsBuilder(t, f.tx, testutil.NewEventCategoryBuilder(t, f.tx, testutil.NewEventBuilder(t, f.tx).Build()).Build())
}

// request は user がログインし、URLの {id} に id を載せたリクエストを作る。form があればフォームの本文にする。
func request(method string, user *model.User, id model.GoodsID, form url.Values) *http.Request {
	target := "/admin/goods/" + id.String() + "/archive"
	var req *http.Request
	if form == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id.String())
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, user)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// unarchive は user として、アーカイブしたグッズを公開に戻すフォームを版 lockVersion で送る。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
func (f *fixture) unarchive(user *model.User, id model.GoodsID, lockVersion string) *httptest.ResponseRecorder {
	form := url.Values{"_method": {http.MethodDelete}, "lock_version": {lockVersion}}
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(f.handler.Delete)).ServeHTTP(rec, request(http.MethodPost, user, id, form))

	return rec
}

// assertContains はボディに wants のすべてが含まれることを検証する。
func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// assertRedirect は応答が location への303であることを検証する。
func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, location string) {
	t.Helper()

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != location {
		t.Fatalf("応答 = %d %q、303 %q を期待", rec.Code, rec.Header().Get("Location"), location)
	}
}

// TestNew は、編集者に、グッズへのパンくずと理由の入力欄を持つアーカイブの画面を描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.goods(t).WithName("くまの子").Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, f.user(t, model.UserRoleEditor), id, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"<title>グッズをアーカイブ | Cutre</title>",
		`href="/admin/goods/`+id.String()+`/edit"`,
		"「くまの子」をアーカイブします。",
		`action="/admin/goods/`+id.String()+`/archive" method="post"`,
		`<textarea id="archive_message" name="archive_message"`,
		`name="lock_version" value="0"`,
	)
}

// TestNew_Archived は、既にアーカイブしたグッズでは編集の画面へ送ることを検証する。
func TestNew_Archived(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.goods(t).WithArchived("景品から外れたため").Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, f.user(t, model.UserRoleEditor), id, nil))

	assertRedirect(t, rec, "/admin/goods/"+id.String()+"/edit")
}

// TestNew_NotFound は、一般のユーザーと削除したグッズに404を返すことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	for name, tt := range map[string]struct {
		user *model.User
		id   model.GoodsID
	}{
		"一般のユーザー": {user: f.user(t, model.UserRoleUser), id: f.goods(t).Build()},
		"削除したグッズ": {user: f.user(t, model.UserRoleEditor), id: f.goods(t).WithDeleted().Build()},
	} {
		rec := httptest.NewRecorder()
		f.handler.New(rec, request(http.MethodGet, tt.user, tt.id, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestCreate は、理由を残してグッズをアーカイブし、編集の画面へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.goods(t).Build()

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {"景品から外れたため"}, "lock_version": {"0"}}))

	assertRedirect(t, rec, "/admin/goods/"+id.String()+"/edit")
	found, _ := f.goodsRepo.FindByID(t.Context(), id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "景品から外れたため" {
		t.Errorf("アーカイブしたグッズ = %+v、理由を残したアーカイブを期待", found)
	}
}

// TestCreate_Invalid は、理由が空のときにアーカイブせず、送った版を持ち回ってエラーを付けて描き直す (422) ことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.goods(t).Build()
	if updated, err := f.goodsRepo.Update(t.Context(), id, 0, repository.GoodsAttributes{Name: "変更後のグッズ", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {" "}, "lock_version": {"0"}}))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), "入力してください", `name="lock_version" value="0"`)
	if found, _ := f.goodsRepo.FindByID(t.Context(), id); found.IsArchived() {
		t.Error("理由が空なのにアーカイブした")
	}
}

// TestCreate_Conflict は、アーカイブの画面を開いたあとに変更されたグッズを古い版からアーカイブしないことを検証する。
func TestCreate_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.goods(t).Build()
	if updated, err := f.goodsRepo.Update(t.Context(), id, 0, repository.GoodsAttributes{Name: "変更後のグッズ", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {"古い理由"}, "lock_version": {"0"}}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	assertContains(t, rec.Body.String(), "変更後のグッズ", "ほかの操作で先に更新されたため", "グッズを確認する")
	if found, _ := f.goodsRepo.FindByID(t.Context(), id); found.IsArchived() {
		t.Error("古い画面からグッズをアーカイブした")
	}
}

// TestDelete は、アーカイブしたグッズを公開に戻して編集の画面へ戻し、一般のユーザーには404を返すことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.goods(t).WithArchived("景品から外れたため").Build()

	if rec := f.unarchive(f.user(t, model.UserRoleUser), id, "0"); rec.Code != http.StatusNotFound {
		t.Fatalf("一般のユーザーのステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}

	assertRedirect(t, f.unarchive(f.user(t, model.UserRoleEditor), id, "0"), "/admin/goods/"+id.String()+"/edit")
	if found, _ := f.goodsRepo.FindByID(t.Context(), id); found.Status != model.MasterStatusPublished {
		t.Errorf("状態 = %q、公開中を期待", found.Status)
	}
}

// TestDelete_Conflict は、公開に戻す画面を開いたあとに変更されたグッズを古い版から戻さないことを検証する。
func TestDelete_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.goods(t).WithArchived("景品から外れたため").Build()
	if updated, err := f.goodsRepo.Update(t.Context(), id, 0, repository.GoodsAttributes{Name: "変更後のグッズ", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := f.unarchive(f.user(t, model.UserRoleEditor), id, "0")
	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	assertContains(t, rec.Body.String(), "変更後のグッズ", "アーカイブ", "ほかの操作で先に更新されたため")
	if found, _ := f.goodsRepo.FindByID(t.Context(), id); !found.IsArchived() {
		t.Error("古い画面からグッズを公開に戻した")
	}
}
