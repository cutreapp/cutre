package admin_goods_test

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
	"github.com/cutreapp/cutre/go/internal/handler/admin_goods"
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

// fixture は Handler と、テストデータを書き込む接続・グッズのリポジトリ。
type fixture struct {
	handler   *admin_goods.Handler
	conn      dbConn
	goodsRepo *repository.GoodsRepository
	userRepo  *repository.UserRepository
}

// dbConn はテストデータを書き込む接続。テスト用のトランザクションか、共有の接続プールのどちらか。
type dbConn interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// fixtureRepos は fixture の Handler が使うリポジトリ。
type fixtureRepos struct {
	event    *repository.EventRepository
	category *repository.EventCategoryRepository
	goods    *repository.GoodsRepository
	item     *repository.ItemRepository
	user     *repository.UserRepository
}

// newFixture はテスト用のトランザクションの中で動く fixture を返す。書いた行はテストの終了時にロールバックされる。
// 削除のUseCaseは自分でトランザクションを開き、このトランザクションで書いた行を読めないため、削除を送るテストでは newCommittedFixture を使う。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	return buildFixture(db, tx, fixtureRepos{
		event:    repository.NewEventRepository(db).WithTx(tx),
		category: repository.NewEventCategoryRepository(db).WithTx(tx),
		goods:    repository.NewGoodsRepository(db).WithTx(tx),
		item:     repository.NewItemRepository(db).WithTx(tx),
		user:     repository.NewUserRepository(db).WithTx(tx),
	})
}

// newCommittedFixture はテスト用のデータベースに直接書き込む fixture を返す (テストデータはコミットされる)。
// 自分でトランザクションを開く削除のUseCaseを通すテストで使う。
func newCommittedFixture(t *testing.T) *fixture {
	t.Helper()

	db := testutil.GetTestDB()
	return buildFixture(db, db, fixtureRepos{
		event:    repository.NewEventRepository(db),
		category: repository.NewEventCategoryRepository(db),
		goods:    repository.NewGoodsRepository(db),
		item:     repository.NewItemRepository(db),
		user:     repository.NewUserRepository(db),
	})
}

// buildFixture は repos を使う Handler を組み立てる。削除のUseCaseには、自分でトランザクションを開くための db を渡す。
func buildFixture(db *sql.DB, conn dbConn, repos fixtureRepos) *fixture {
	eventRepo, categoryRepo, goodsRepo, itemRepo := repos.event, repos.category, repos.goods, repos.item
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	return &fixture{
		handler: admin_goods.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetAdminEventCategoryUsecase(eventRepo, categoryRepo, goodsRepo),
			usecase.NewGetAdminGoodsUsecase(eventRepo, categoryRepo, goodsRepo),
			usecase.NewCreateGoodsUsecase(validator.NewGoodsCreateValidator(), eventRepo, categoryRepo, goodsRepo),
			usecase.NewUpdateGoodsUsecase(validator.NewGoodsUpdateValidator(), eventRepo, categoryRepo, goodsRepo),
			usecase.NewDeleteGoodsUsecase(db, validator.NewGoodsDeleteValidator(itemRepo), eventRepo, categoryRepo, goodsRepo),
		),
		conn:      conn,
		goodsRepo: goodsRepo,
		userRepo:  repos.user,
	}
}

// user は role の役割のユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func (f *fixture) user(t *testing.T, role model.UserRole) *model.User {
	t.Helper()

	id := testutil.NewUserBuilder(t, f.conn).WithRole(role).Build()
	user, err := f.userRepo.FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// request は user がログインしたリクエストを作る。URLの {id} に id を載せ、form があればフォームの本文にする。
func request(method, target string, user *model.User, id string, form url.Values) *http.Request {
	var req *http.Request
	if form == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, user)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// deleteGoods は user として、グッズの削除のフォームを版 lockVersion で送る。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
func (f *fixture) deleteGoods(user *model.User, id model.GoodsID, lockVersion string) *httptest.ResponseRecorder {
	form := url.Values{"_method": {http.MethodDelete}, "lock_version": {lockVersion}}
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(f.handler.Delete)).ServeHTTP(rec, request(http.MethodPost, "/admin/goods/"+id.String(), user, id.String(), form))

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

// assertStatus は応答のステータスコードが want であることを検証する。
func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, want)
	}
}

// assertRedirect は応答が location への303であることを検証する。
func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, location string) {
	t.Helper()

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != location {
		t.Fatalf("応答 = %d %q、303 %q を期待", rec.Code, rec.Header().Get("Location"), location)
	}
}

// newCategory はテスト用のイベントの配下にカテゴリーを作り、カテゴリーのIDを返す。
func (f *fixture) newCategory(t *testing.T) model.EventCategoryID {
	t.Helper()

	return testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).Build()).Build()
}

// TestNew は、編集者に、イベントとカテゴリーへのパンくずと、既存のグッズの最後の並び順に100を足した値を入れた作成のフォームを描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	eventID := testutil.NewEventBuilder(t, f.conn).WithName("秋のくじ").Build()
	categoryID := testutil.NewEventCategoryBuilder(t, f.conn, eventID).WithName("A賞").Build()
	testutil.NewGoodsBuilder(t, f.conn, categoryID).WithPosition(4).Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, "/admin/categories/"+categoryID.String()+"/goods/new", f.user(t, model.UserRoleEditor), categoryID.String(), nil))

	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec.Body.String(),
		"<title>グッズを作成 | Cutre</title>",
		`href="/admin/events/`+eventID.String()+`/edit"`,
		`href="/admin/categories/`+categoryID.String()+`/edit"`,
		"秋のくじ",
		"A賞",
		`action="/admin/categories/`+categoryID.String()+`/goods" method="post"`,
		`value="104"`,
	)
}

// TestNew_NotFound は、一般のユーザー・削除したカテゴリー・削除したイベントのカテゴリーに404を返すことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	editor := f.user(t, model.UserRoleEditor)
	for name, tt := range map[string]struct {
		user       *model.User
		categoryID string
	}{
		"一般のユーザー":        {user: f.user(t, model.UserRoleUser), categoryID: f.newCategory(t).String()},
		"削除したカテゴリー":      {user: editor, categoryID: testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).Build()).WithDeleted().Build().String()},
		"削除したイベントのカテゴリー": {user: editor, categoryID: testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).WithDeleted().Build()).Build().String()},
	} {
		rec := httptest.NewRecorder()
		f.handler.New(rec, request(http.MethodGet, "/admin/categories/"+tt.categoryID+"/goods/new", tt.user, tt.categoryID, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestCreate は、グッズを作成し、完了のメッセージを付けてカテゴリーの編集の画面へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	categoryID := f.newCategory(t)
	form := url.Values{"name": {"くまの子"}, "position": {"1"}}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/categories/"+categoryID.String()+"/goods", f.user(t, model.UserRoleEditor), categoryID.String(), form))

	assertRedirect(t, rec, "/admin/categories/"+categoryID.String()+"/edit")
	goods, err := f.goodsRepo.ListUndeletedByEventCategoryID(t.Context(), categoryID)
	if err != nil || len(goods) != 1 || goods[0].Name != "くまの子" {
		t.Errorf("カテゴリーのグッズ = (%v, %v)、「くまの子」を1つ期待", goods, err)
	}
}

// TestCreate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直す (422) ことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	categoryID := f.newCategory(t)
	form := url.Values{"name": {""}, "position": {"2"}}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/categories/"+categoryID.String()+"/goods", f.user(t, model.UserRoleEditor), categoryID.String(), form))

	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), "入力してください", `value="2"`)
}

// TestEdit は、編集者に、カテゴリーへのパンくずと保存済みの値・版を入れた編集のフォーム・アーカイブの画面へのリンクを描画し、
// 削除の欄は出さないことを検証する。
func TestEdit(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	categoryID := f.newCategory(t)
	id := testutil.NewGoodsBuilder(t, f.conn, categoryID).WithName("くまの子").WithPosition(3).Build()

	rec := httptest.NewRecorder()
	f.handler.Edit(rec, request(http.MethodGet, "/admin/goods/"+id.String()+"/edit", f.user(t, model.UserRoleEditor), id.String(), nil))

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body,
		`<h1 class="text-xl font-semibold">くまの子</h1>`,
		`href="/admin/categories/`+categoryID.String()+`/edit"`,
		`action="/admin/goods/`+id.String()+`" method="post"`,
		`<input type="hidden" name="lock_version" value="0">`,
		`value="3"`,
		`href="/admin/goods/`+id.String()+`/archive/new"`,
	)
	if strings.Contains(body, "goods-delete-dialog") {
		t.Error("編集者に削除の欄を出している")
	}
}

// TestEdit_Admin は、管理者には削除の欄を出し、アーカイブしたグッズには理由と元に戻すフォームを出すことを検証する。
func TestEdit_Admin(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewGoodsBuilder(t, f.conn, f.newCategory(t)).WithName("うさぎの子").WithArchived("景品から外れたため").Build()

	rec := httptest.NewRecorder()
	f.handler.Edit(rec, request(http.MethodGet, "/admin/goods/"+id.String()+"/edit", f.user(t, model.UserRoleAdmin), id.String(), nil))

	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec.Body.String(),
		"景品から外れたため",
		`action="/admin/goods/`+id.String()+`/archive" method="post"`,
		">元に戻す<",
		`commandfor="goods-delete-dialog" command="show-modal"`,
		"「うさぎの子」を削除します。元に戻せません。",
	)
}

// TestEdit_NotFound は、一般のユーザー・削除したグッズ・削除したカテゴリーのグッズ・UUIDとして読めないIDに404を返すことを検証する。
func TestEdit_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	editor := f.user(t, model.UserRoleEditor)
	categoryID := f.newCategory(t)
	deletedCategoryID := testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).Build()).WithDeleted().Build()
	for name, tt := range map[string]struct {
		user *model.User
		id   string
	}{
		"一般のユーザー":       {user: f.user(t, model.UserRoleUser), id: testutil.NewGoodsBuilder(t, f.conn, categoryID).Build().String()},
		"削除したグッズ":       {user: editor, id: testutil.NewGoodsBuilder(t, f.conn, categoryID).WithDeleted().Build().String()},
		"削除したカテゴリーのグッズ": {user: editor, id: testutil.NewGoodsBuilder(t, f.conn, deletedCategoryID).Build().String()},
		"読めないID":        {user: editor, id: "not-a-uuid"},
	} {
		rec := httptest.NewRecorder()
		f.handler.Edit(rec, request(http.MethodGet, "/admin/goods/"+tt.id+"/edit", tt.user, tt.id, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestUpdate は、グッズを更新し、完了のメッセージを付けてカテゴリーの編集の画面へ戻すことを検証する。
func TestUpdate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	categoryID := f.newCategory(t)
	id := testutil.NewGoodsBuilder(t, f.conn, categoryID).Build()
	form := url.Values{"lock_version": {"0"}, "name": {"更新したグッズ"}, "position": {"7"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/goods/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertRedirect(t, rec, "/admin/categories/"+categoryID.String()+"/edit")
	found, _ := f.goodsRepo.FindByID(t.Context(), id)
	if found.Name != "更新したグッズ" || found.Position != 7 || found.LockVersion != 1 {
		t.Errorf("更新したグッズ = %+v、「更新したグッズ」・並び順7・版1を期待", found)
	}
}

// TestUpdate_Conflict は、フォームを開いたあとにほかの操作で先に更新されていたら上書きせず、
// 最新の値と版で編集の画面を描き直す (409) ことを検証する。
func TestUpdate_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewGoodsBuilder(t, f.conn, f.newCategory(t)).Build()
	if updated, err := f.goodsRepo.Update(t.Context(), id, 0, repository.GoodsAttributes{Name: "先に更新されたグッズ", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}
	form := url.Values{"lock_version": {"0"}, "name": {"古い版からのグッズ"}, "position": {"2"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/goods/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertStatus(t, rec, http.StatusConflict)
	body := rec.Body.String()
	assertContains(t, body, "ほかの操作で先に更新されたため、保存しませんでした", `value="先に更新されたグッズ"`, `name="lock_version" value="1"`)
	if strings.Contains(body, "古い版からのグッズ") {
		t.Error("競合した送信の値をフォームに戻している")
	}
}

// TestUpdate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直す (422) ことを検証する。
func TestUpdate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewGoodsBuilder(t, f.conn, f.newCategory(t)).Build()
	form := url.Values{"lock_version": {"0"}, "name": {"くまの子"}, "position": {"x"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/goods/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), "0から9999までの整数を入力してください", `value="x"`)
}

// TestDelete は、管理者がグッズを削除してカテゴリーの編集の画面へ戻れ、編集者には404を返して削除しないことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	categoryID := f.newCategory(t)
	id := testutil.NewGoodsBuilder(t, f.conn, categoryID).Build()

	assertStatus(t, f.deleteGoods(f.user(t, model.UserRoleEditor), id, "0"), http.StatusNotFound)

	assertRedirect(t, f.deleteGoods(f.user(t, model.UserRoleAdmin), id, "0"), "/admin/categories/"+categoryID.String()+"/edit")
	if found, _ := f.goodsRepo.FindByID(t.Context(), id); !found.IsDeleted() {
		t.Errorf("状態 = %q、削除した状態を期待", found.Status)
	}
}

// TestDelete_Conflict は、確認の画面を開いたあとに変更されたグッズを古い版から削除せず、最新の値で描き直す (409) ことを検証する。
func TestDelete_Conflict(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	id := testutil.NewGoodsBuilder(t, f.conn, f.newCategory(t)).Build()
	if updated, err := f.goodsRepo.Update(t.Context(), id, 0, repository.GoodsAttributes{Name: "変更後のグッズ", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := f.deleteGoods(f.user(t, model.UserRoleAdmin), id, "0")

	assertStatus(t, rec, http.StatusConflict)
	assertContains(t, rec.Body.String(), "変更後のグッズ", "ほかの操作で先に更新されたため", `name="lock_version" value="1"`)
	if found, _ := f.goodsRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("古い画面からグッズを削除した")
	}
}

// TestDelete_Referenced は、リストのアイテムから参照されているグッズを削除せず、
// アーカイブを案内するエラーを付けて編集の画面を描き直すことを検証する。
func TestDelete_Referenced(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	admin := f.user(t, model.UserRoleAdmin)
	id := testutil.NewGoodsBuilder(t, f.conn, f.newCategory(t)).Build()
	testutil.NewItemBuilder(t, f.conn, admin.ID, id).WithRemoved().Build()

	rec := f.deleteGoods(admin, id, "0")
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), "このグッズはリストに入れられたことがあるため、削除できません。代わりにアーカイブしてください")
	if found, _ := f.goodsRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("アイテムから参照されているグッズを削除した")
	}
}
