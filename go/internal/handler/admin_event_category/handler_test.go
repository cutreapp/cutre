package admin_event_category_test

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
	"github.com/cutreapp/cutre/go/internal/handler/admin_event_category"
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

// fixture は Handler と、テストデータを書き込む接続・カテゴリーのリポジトリ。
type fixture struct {
	handler      *admin_event_category.Handler
	conn         dbConn
	categoryRepo *repository.EventCategoryRepository
	userRepo     *repository.UserRepository
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
		handler: admin_event_category.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetAdminEventUsecase(eventRepo, categoryRepo),
			usecase.NewGetAdminEventCategoryUsecase(eventRepo, categoryRepo, goodsRepo),
			usecase.NewCreateEventCategoryUsecase(validator.NewEventCategoryCreateValidator(), eventRepo, categoryRepo),
			usecase.NewUpdateEventCategoryUsecase(validator.NewEventCategoryUpdateValidator(), eventRepo, categoryRepo),
			usecase.NewDeleteEventCategoryUsecase(db, validator.NewEventCategoryDeleteValidator(itemRepo), eventRepo, categoryRepo),
		),
		conn:         conn,
		categoryRepo: categoryRepo,
		userRepo:     repos.user,
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

// deleteCategory は user として、カテゴリーの削除のフォームを版 lockVersion で送る。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
func (f *fixture) deleteCategory(user *model.User, id model.EventCategoryID, lockVersion string) *httptest.ResponseRecorder {
	form := url.Values{"_method": {http.MethodDelete}, "lock_version": {lockVersion}}
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(f.handler.Delete)).ServeHTTP(rec, request(http.MethodPost, "/admin/categories/"+id.String(), user, id.String(), form))

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

// TestNew は、編集者に、イベントへのパンくずと、既存のカテゴリーの最後の並び順に100を足した値を入れた作成のフォームを描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	eventID := testutil.NewEventBuilder(t, f.conn).WithName("秋のくじ").Build()
	testutil.NewEventCategoryBuilder(t, f.conn, eventID).WithPosition(10).Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, "/admin/events/"+eventID.String()+"/categories/new", f.user(t, model.UserRoleEditor), eventID.String(), nil))

	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec.Body.String(),
		"<title>カテゴリーを作成 | Cutre</title>",
		`href="/admin/events/`+eventID.String()+`/edit"`,
		"秋のくじ",
		`action="/admin/events/`+eventID.String()+`/categories" method="post"`,
		`name="position" type="number"`,
		`value="110"`,
	)
}

// TestNew_NotFound は、一般のユーザー・削除したイベント・UUIDとして読めないIDに404を返すことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	editor := f.user(t, model.UserRoleEditor)
	for name, tt := range map[string]struct {
		user    *model.User
		eventID string
	}{
		"一般のユーザー":  {user: f.user(t, model.UserRoleUser), eventID: testutil.NewEventBuilder(t, f.conn).Build().String()},
		"削除したイベント": {user: editor, eventID: testutil.NewEventBuilder(t, f.conn).WithDeleted().Build().String()},
		"読めないID":   {user: editor, eventID: "not-a-uuid"},
	} {
		rec := httptest.NewRecorder()
		f.handler.New(rec, request(http.MethodGet, "/admin/events/"+tt.eventID+"/categories/new", tt.user, tt.eventID, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestCreate は、カテゴリーを作成し、完了のメッセージを付けてイベントの編集の画面へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	eventID := testutil.NewEventBuilder(t, f.conn).Build()
	form := url.Values{"name": {"A賞"}, "position": {"1"}}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/events/"+eventID.String()+"/categories", f.user(t, model.UserRoleEditor), eventID.String(), form))

	assertRedirect(t, rec, "/admin/events/"+eventID.String()+"/edit")
	categories, err := f.categoryRepo.ListUndeletedByEventID(t.Context(), eventID)
	if err != nil || len(categories) != 1 || categories[0].Name != "A賞" {
		t.Errorf("イベントのカテゴリー = (%v, %v)、「A賞」を1つ期待", categories, err)
	}
}

// TestCreate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直す (422) ことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	eventID := testutil.NewEventBuilder(t, f.conn).Build()
	form := url.Values{"name": {"A賞"}, "position": {"-1"}}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/events/"+eventID.String()+"/categories", f.user(t, model.UserRoleEditor), eventID.String(), form))

	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), `value="A賞"`, `value="-1"`, "0から9999までの整数を入力してください")
}

// TestEdit は、編集者に、保存済みの値と版を入れた編集のフォーム・グッズの一覧・アーカイブの画面へのリンクを描画し、
// 削除の欄は出さないことを検証する。
func TestEdit(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	eventID := testutil.NewEventBuilder(t, f.conn).WithName("秋のくじ").Build()
	id := testutil.NewEventCategoryBuilder(t, f.conn, eventID).WithName("A賞").WithPosition(3).Build()
	goodsID := testutil.NewGoodsBuilder(t, f.conn, id).WithName("くまの子").Build()
	testutil.NewGoodsBuilder(t, f.conn, id).WithName("うさぎの子").WithArchived("景品から外れたため").Build()
	testutil.NewGoodsBuilder(t, f.conn, id).WithName("削除したグッズ").WithDeleted().Build()

	rec := httptest.NewRecorder()
	f.handler.Edit(rec, request(http.MethodGet, "/admin/categories/"+id.String()+"/edit", f.user(t, model.UserRoleEditor), id.String(), nil))

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body,
		`<h1 class="text-xl font-semibold">A賞</h1>`,
		`href="/admin/events/`+eventID.String()+`/edit"`,
		`action="/admin/categories/`+id.String()+`" method="post"`,
		`<input type="hidden" name="_method" value="PATCH">`,
		`<input type="hidden" name="lock_version" value="0">`,
		`value="3"`,
		`href="/admin/goods/`+goodsID.String()+`/edit"`,
		"くまの子",
		"うさぎの子",
		`href="/admin/categories/`+id.String()+`/goods/new"`,
		`href="/admin/categories/`+id.String()+`/archive/new"`,
	)
	if strings.Contains(body, "削除したグッズ") || strings.Contains(body, "event-category-delete-dialog") {
		t.Error("削除したグッズか、編集者に削除の欄を出している")
	}
}

// TestEdit_Admin は、管理者には削除の欄を出し、アーカイブしたカテゴリーには理由と元に戻すフォームを出すことを検証する。
func TestEdit_Admin(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).Build()).WithName("B賞").WithArchived("景品が変わったため").Build()

	rec := httptest.NewRecorder()
	f.handler.Edit(rec, request(http.MethodGet, "/admin/categories/"+id.String()+"/edit", f.user(t, model.UserRoleAdmin), id.String(), nil))

	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec.Body.String(),
		"景品が変わったため",
		`action="/admin/categories/`+id.String()+`/archive" method="post"`,
		">元に戻す<",
		`commandfor="event-category-delete-dialog" command="show-modal"`,
		"「B賞」を削除します。元に戻せません。",
	)
}

// TestEdit_NotFound は、一般のユーザー・削除したカテゴリー・削除したイベントのカテゴリー・UUIDとして読めないIDに404を返すことを検証する。
func TestEdit_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	editor := f.user(t, model.UserRoleEditor)
	eventID := testutil.NewEventBuilder(t, f.conn).Build()
	for name, tt := range map[string]struct {
		user *model.User
		id   string
	}{
		"一般のユーザー":        {user: f.user(t, model.UserRoleUser), id: testutil.NewEventCategoryBuilder(t, f.conn, eventID).Build().String()},
		"削除したカテゴリー":      {user: editor, id: testutil.NewEventCategoryBuilder(t, f.conn, eventID).WithDeleted().Build().String()},
		"削除したイベントのカテゴリー": {user: editor, id: testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).WithDeleted().Build()).Build().String()},
		"読めないID":         {user: editor, id: "not-a-uuid"},
	} {
		rec := httptest.NewRecorder()
		f.handler.Edit(rec, request(http.MethodGet, "/admin/categories/"+tt.id+"/edit", tt.user, tt.id, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestUpdate は、カテゴリーを更新し、完了のメッセージを付けてイベントの編集の画面へ戻すことを検証する。
func TestUpdate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	eventID := testutil.NewEventBuilder(t, f.conn).Build()
	id := testutil.NewEventCategoryBuilder(t, f.conn, eventID).Build()
	form := url.Values{"lock_version": {"0"}, "name": {"更新したカテゴリー"}, "position": {"7"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/categories/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertRedirect(t, rec, "/admin/events/"+eventID.String()+"/edit")
	found, _ := f.categoryRepo.FindByID(t.Context(), id)
	if found.Name != "更新したカテゴリー" || found.Position != 7 || found.LockVersion != 1 {
		t.Errorf("更新したカテゴリー = %+v、「更新したカテゴリー」・並び順7・版1を期待", found)
	}
}

// TestUpdate_Conflict は、フォームを開いたあとにほかの操作で先に更新されていたら上書きせず、
// 最新の値と版で編集の画面を描き直す (409) ことを検証する。
func TestUpdate_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).Build()).Build()
	if updated, err := f.categoryRepo.Update(t.Context(), id, 0, repository.EventCategoryAttributes{Name: "先に更新されたカテゴリー", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}
	form := url.Values{"lock_version": {"0"}, "name": {"古い版からのカテゴリー"}, "position": {"2"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/categories/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertStatus(t, rec, http.StatusConflict)
	body := rec.Body.String()
	assertContains(t, body, "ほかの操作で先に更新されたため、保存しませんでした", `value="先に更新されたカテゴリー"`, `name="lock_version" value="1"`)
	if strings.Contains(body, "古い版からのカテゴリー") {
		t.Error("競合した送信の値をフォームに戻している")
	}
}

// TestUpdate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直す (422) ことを検証する。
func TestUpdate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).Build()).Build()
	form := url.Values{"lock_version": {"0"}, "name": {""}, "position": {"5"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/categories/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), "入力してください", `value="5"`)
}

// TestDelete は、管理者がカテゴリーを削除してイベントの編集の画面へ戻れ、編集者には404を返して削除しないことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	eventID := testutil.NewEventBuilder(t, f.conn).Build()
	id := testutil.NewEventCategoryBuilder(t, f.conn, eventID).Build()

	assertStatus(t, f.deleteCategory(f.user(t, model.UserRoleEditor), id, "0"), http.StatusNotFound)

	assertRedirect(t, f.deleteCategory(f.user(t, model.UserRoleAdmin), id, "0"), "/admin/events/"+eventID.String()+"/edit")
	if found, _ := f.categoryRepo.FindByID(t.Context(), id); !found.IsDeleted() {
		t.Errorf("状態 = %q、削除した状態を期待", found.Status)
	}
}

// TestDelete_Conflict は、確認の画面を開いたあとに変更されたカテゴリーを古い版から削除せず、最新の値で描き直す (409) ことを検証する。
func TestDelete_Conflict(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	id := testutil.NewEventCategoryBuilder(t, f.conn, testutil.NewEventBuilder(t, f.conn).Build()).Build()
	if updated, err := f.categoryRepo.Update(t.Context(), id, 0, repository.EventCategoryAttributes{Name: "変更後のカテゴリー", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := f.deleteCategory(f.user(t, model.UserRoleAdmin), id, "0")

	assertStatus(t, rec, http.StatusConflict)
	assertContains(t, rec.Body.String(), "変更後のカテゴリー", "ほかの操作で先に更新されたため", `name="lock_version" value="1"`)
	if found, _ := f.categoryRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("古い画面からカテゴリーを削除した")
	}
}
