package item_test

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
	"github.com/cutreapp/cutre/go/internal/handler/item"
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

// fixture はテスト用のデータベースに直接書き込む Handler と、その接続・ログイン中のユーザー・公開中のグッズ。
// リストに追加するUseCaseが自分でトランザクションを開くため、テストのトランザクションではなく
// GetTestDB の接続を使う (テストデータはコミットされる)。テストごとにユーザーとグッズを作るため、テストどうしは干渉しない。
type fixture struct {
	handler    *item.Handler
	db         *sql.DB
	user       *model.User
	eventID    model.EventID
	categoryID model.EventCategoryID
	goodsID    model.GoodsID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db := testutil.GetTestDB()
	eventRepo := repository.NewEventRepository(db)
	categoryRepo := repository.NewEventCategoryRepository(db)
	goodsRepo := repository.NewGoodsRepository(db)
	itemRepo := repository.NewItemRepository(db)
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	userID := testutil.NewUserBuilder(t, db).Build()
	user, err := repository.NewUserRepository(db).FindByID(context.Background(), userID)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}
	eventID := testutil.NewEventBuilder(t, db).WithName("ふわりす もちもちくじ").Build()
	categoryID := testutil.NewEventCategoryBuilder(t, db, eventID).WithName("B賞").Build()

	return &fixture{
		handler: item.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetGoodsUsecase(eventRepo, categoryRepo, goodsRepo),
			usecase.NewCreateItemUsecase(db, validator.NewItemCreateValidator(), eventRepo, categoryRepo, goodsRepo, itemRepo),
			usecase.NewGetItemUsecase(eventRepo, categoryRepo, goodsRepo, itemRepo),
			usecase.NewUpdateItemUsecase(validator.NewItemUpdateValidator(), itemRepo),
			usecase.NewDeleteItemUsecase(itemRepo),
		),
		db:         db,
		user:       user,
		eventID:    eventID,
		categoryID: categoryID,
		goodsID:    testutil.NewGoodsBuilder(t, db, categoryID).WithName("くまの子").Build(),
	}
}

// request はログイン中のユーザーのリクエストを作る。form があればフォームの本文にする。
func (f *fixture) request(method, target string, form url.Values) *http.Request {
	return f.requestForItem(method, target, "", form)
}

// requestForItem は、URLの {id} に itemID を入れた、ログイン中のユーザーのリクエストを作る。
func (f *fixture) requestForItem(method, target, itemID string, form url.Values) *http.Request {
	var req *http.Request
	if form == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, f.user)
	if itemID != "" {
		routeCtx := chi.NewRouteContext()
		routeCtx.URLParams.Add("id", itemID)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
	}

	return req.WithContext(ctx)
}

// listedItems はログイン中のユーザーの、グッズのカテゴリーのリストにあるアイテムを返す。
func (f *fixture) listedItems(t *testing.T) []*model.Item {
	t.Helper()

	items, err := repository.NewItemRepository(f.db).ListListedByUserIDAndEventCategoryID(context.Background(), f.user.ID, f.categoryID)
	if err != nil {
		t.Fatalf("アイテムの取得のエラー = %v", err)
	}

	return items
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

// TestNew は、グッズと、そのイベント・カテゴリーへ戻るパンくずを出し、クエリで選んだリストを選んだ状態でフォームを描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.New(rec, f.request(http.MethodGet, "/items/new?goods_id="+f.goodsID.String()+"&kind=want", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"くまの子",
		`href="/events/`+f.eventID.String()+`/categories/`+f.categoryID.String()+`"`,
		`action="/items"`,
		`name="goods_id" value="`+f.goodsID.String()+`"`,
		`value="want" class="sr-only" required checked`,
		`name="quantity" type="number" inputmode="numeric" min="1" max="99" step="1" value="1"`,
	)
}

// TestNew_NotFound は、UUIDとして読めないグッズと、公開していないグッズに404を返すことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	archivedID := testutil.NewGoodsBuilder(t, f.db, f.categoryID).WithArchived("景品から外れたため").Build()

	for name, id := range map[string]string{"UUIDとして読めない": "not-a-uuid", "アーカイブしたグッズ": archivedID.String()} {
		rec := httptest.NewRecorder()
		f.handler.New(rec, f.request(http.MethodGet, "/items/new?goods_id="+id+"&kind=give", nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestCreate は、アイテムをリストに入れ、完了のメッセージを付けてカテゴリーのグッズの一覧へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	form := url.Values{"goods_id": {f.goodsID.String()}, "kind": {"give"}, "quantity": {"2"}, "note": {"未開封"}}
	rec := httptest.NewRecorder()
	f.handler.Create(rec, f.request(http.MethodPost, "/items", form))

	wantLocation := "/events/" + f.eventID.String() + "/categories/" + f.categoryID.String()
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != wantLocation {
		t.Fatalf("応答 = %d %q、303 %q を期待", rec.Code, rec.Header().Get("Location"), wantLocation)
	}
	assertFlashSet(t, rec)
	items := f.listedItems(t)
	if len(items) != 1 || items[0].GoodsID != f.goodsID || items[0].Kind != model.ItemKindGive || items[0].Quantity != 2 || items[0].Note != "未開封" {
		t.Errorf("リストのアイテム = %+v、譲れるリストに2点・「未開封」を期待", items)
	}
}

// TestCreate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直し (422)、アイテムを作らないことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	form := url.Values{"goods_id": {f.goodsID.String()}, "kind": {"want"}, "quantity": {"0"}, "note": {"色違いでも可"}}
	rec := httptest.NewRecorder()
	f.handler.Create(rec, f.request(http.MethodPost, "/items", form))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), "1から99までの整数を入力してください", `value="want" class="sr-only" required checked`, "色違いでも可", `value="0"`)
	if items := f.listedItems(t); len(items) != 0 {
		t.Errorf("リストのアイテム = %+v、作らないことを期待", items)
	}
}

// TestCreate_NotFound は、公開していないグッズをリストに入れず、404を返すことを検証する。
func TestCreate_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	archivedID := testutil.NewGoodsBuilder(t, f.db, f.categoryID).WithArchived("景品から外れたため").Build()
	form := url.Values{"goods_id": {archivedID.String()}, "kind": {"give"}, "quantity": {"1"}}
	rec := httptest.NewRecorder()
	f.handler.Create(rec, f.request(http.MethodPost, "/items", form))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// assertFlashSet は、完了のメッセージがCookieに書き込まれたことを検証する。
func assertFlashSet(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name == session.FlashCookieName && c.Value != "" {
			return
		}
	}
	t.Error("完了のメッセージが書き込まれていない")
}

// findItem はアイテムを状態を問わずに引く。
func (f *fixture) findItem(t *testing.T, id model.ItemID) *model.Item {
	t.Helper()

	item, err := repository.NewItemRepository(f.db).FindByID(context.Background(), id)
	if err != nil || item == nil {
		t.Fatalf("アイテムの取得 = (%v, %v)、アイテムを期待", item, err)
	}

	return item
}

// TestEdit は、アイテムのグッズと、リストへ戻るパンくずを出し、保存済みの数量とひとことを入れた
// 更新のフォームと、リストから外すフォームを描画することを検証する。
func TestEdit(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	itemID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).WithKind(model.ItemKindWant).WithQuantity(3).WithNote("色違いでも可").Build()
	rec := httptest.NewRecorder()
	f.handler.Edit(rec, f.requestForItem(http.MethodGet, "/items/"+itemID.String()+"/edit", itemID.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"ほしいリストのアイテム",
		`href="/list?kind=want"`,
		"ふわりす もちもちくじ・B賞",
		"くまの子",
		`action="/items/`+itemID.String()+`"`,
		`name="_method" value="PATCH"`,
		`name="_method" value="DELETE"`,
		`name="lock_version" value="0"`,
		`value="3"`,
		">色違いでも可</textarea>",
		"リストから外す",
		`href="/list" class=`,
	)
}

// TestEdit_NotFound は、UUIDとして読めないID・外したアイテム・ほかのユーザーのアイテムに404を返すことを検証する。
func TestEdit_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	removedID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).WithRemoved().Build()
	otherID := testutil.NewItemBuilder(t, f.db, testutil.NewUserBuilder(t, f.db).Build(), f.goodsID).Build()

	for name, id := range map[string]string{"UUIDとして読めない": "not-a-uuid", "外したアイテム": removedID.String(), "ほかのユーザーのアイテム": otherID.String()} {
		rec := httptest.NewRecorder()
		f.handler.Edit(rec, f.requestForItem(http.MethodGet, "/items/"+id+"/edit", id, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestUpdate は、アイテムの数量とひとことを更新し、完了のメッセージを付けてアイテムのリストへ戻すことを検証する。
func TestUpdate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	itemID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).WithKind(model.ItemKindWant).Build()
	form := url.Values{"_method": {"PATCH"}, "lock_version": {"0"}, "quantity": {"4"}, "note": {"未開封"}}
	rec := httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, "/items/"+itemID.String(), itemID.String(), form))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/list?kind=want" {
		t.Fatalf("応答 = %d %q、303 %q を期待", rec.Code, rec.Header().Get("Location"), "/list?kind=want")
	}
	assertFlashSet(t, rec)
	if item := f.findItem(t, itemID); item.Quantity != 4 || item.Note != "未開封" {
		t.Errorf("更新後のアイテム = %+v、4点・「未開封」を期待", item)
	}
}

// TestUpdate_Conflict は二つの編集画面を開いたあと、古い画面からの保存が先の変更を上書きせず、
// 最新の値と版で編集画面を描き直すことを検証する。
func TestUpdate_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	itemID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).WithQuantity(2).Build()
	path := "/items/" + itemID.String()
	for range 2 {
		rec := httptest.NewRecorder()
		f.handler.Edit(rec, f.requestForItem(http.MethodGet, path+"/edit", itemID.String(), nil))
		if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `name="lock_version" value="0"`) != 2 {
			t.Fatalf("編集画面 = %d、更新と削除の両フォームに版0を期待", rec.Code)
		}
	}

	first := url.Values{"lock_version": {"0"}, "quantity": {"3"}, "note": {"先に保存"}}
	rec := httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, path, itemID.String(), first))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("先の更新 = %d、303を期待", rec.Code)
	}

	stale := url.Values{"lock_version": {"0"}, "quantity": {"2"}, "note": {"古い画面"}}
	rec = httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, path, itemID.String(), stale))
	if rec.Code != http.StatusConflict {
		t.Fatalf("古い画面からの更新 = %d、409を期待", rec.Code)
	}
	assertContains(t, rec.Body.String(), "先に変更されたため、操作しませんでした", `value="3"`, "先に保存", `name="lock_version" value="1"`)
	if item := f.findItem(t, itemID); item.Quantity != 3 || item.Note != "先に保存" || item.LockVersion != 1 {
		t.Errorf("アイテム = %+v、先の更新と版1を維持することを期待", item)
	}
}

// TestUpdate_MissingLockVersion は版を持たない送信を競合として拒むことを検証する。
func TestUpdate_MissingLockVersion(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	itemID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).Build()
	rec := httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, "/items/"+itemID.String(), itemID.String(), url.Values{"quantity": {"2"}}))
	if rec.Code != http.StatusConflict {
		t.Errorf("版の無い更新 = %d、409を期待", rec.Code)
	}
	if item := f.findItem(t, itemID); item.Quantity != 1 {
		t.Errorf("アイテム = %+v、変更しないことを期待", item)
	}
}

// TestUpdate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直し (422)、アイテムを更新しないことを検証する。
func TestUpdate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	itemID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).WithQuantity(2).Build()
	form := url.Values{"_method": {"PATCH"}, "lock_version": {"0"}, "quantity": {"100"}, "note": {"箱なし"}}
	rec := httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, "/items/"+itemID.String(), itemID.String(), form))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), "譲れるリストのアイテム", "1から99までの整数を入力してください", `value="100"`, ">箱なし</textarea>")
	if item := f.findItem(t, itemID); item.Quantity != 2 || item.Note != "" {
		t.Errorf("アイテム = %+v、更新しないことを期待", item)
	}
}

// TestUpdate_InvalidStaleVersion は入力エラーの再表示で、更新と削除のフォームに古い版を残すことを検証する。
func TestUpdate_InvalidStaleVersion(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	itemID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).Build()
	path := "/items/" + itemID.String()
	first := url.Values{"lock_version": {"0"}, "quantity": {"2"}}
	rec := httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, path, itemID.String(), first))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("先の更新 = %d、303を期待", rec.Code)
	}

	stale := url.Values{"lock_version": {"0"}, "quantity": {"100"}}
	rec = httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, path, itemID.String(), stale))
	if rec.Code != http.StatusUnprocessableEntity || strings.Count(rec.Body.String(), `name="lock_version" value="0"`) != 2 {
		t.Errorf("古い画面の入力エラー = %d、両フォームが元の版0を保つ422を期待", rec.Code)
	}
}

// TestUpdate_NotFound は、ほかのユーザーのアイテムを更新せず、404を返すことを検証する。
func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	otherID := testutil.NewItemBuilder(t, f.db, testutil.NewUserBuilder(t, f.db).Build(), f.goodsID).Build()
	form := url.Values{"_method": {"PATCH"}, "lock_version": {"0"}, "quantity": {"5"}}
	rec := httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, "/items/"+otherID.String(), otherID.String(), form))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if item := f.findItem(t, otherID); item.Quantity != 1 {
		t.Errorf("ほかのユーザーのアイテム = %+v、更新しないことを期待", item)
	}
}

// TestDelete は、アイテムを消さずにリストから外し、完了のメッセージを付けてアイテムのあったリストへ戻すことと、
// 外したアイテムをもう一度外そうとすると404を返すことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	itemID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).Build()
	form := url.Values{"_method": {"DELETE"}, "lock_version": {"0"}}
	rec := httptest.NewRecorder()
	f.handler.Delete(rec, f.requestForItem(http.MethodPost, "/items/"+itemID.String(), itemID.String(), form))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/list?kind=give" {
		t.Fatalf("応答 = %d %q、303 %q を期待", rec.Code, rec.Header().Get("Location"), "/list?kind=give")
	}
	assertFlashSet(t, rec)
	if item := f.findItem(t, itemID); item.Status != model.ItemStatusRemoved {
		t.Errorf("アイテム = %+v、外した状態を期待", item)
	}

	rec = httptest.NewRecorder()
	f.handler.Delete(rec, f.requestForItem(http.MethodPost, "/items/"+itemID.String(), itemID.String(), form))
	if rec.Code != http.StatusNotFound {
		t.Errorf("2回目のステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestDelete_Conflict は、古い画面からは変更済みのアイテムをリストから外せないことを検証する。
func TestDelete_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	itemID := testutil.NewItemBuilder(t, f.db, f.user.ID, f.goodsID).Build()
	path := "/items/" + itemID.String()
	rec := httptest.NewRecorder()
	f.handler.Edit(rec, f.requestForItem(http.MethodGet, path+"/edit", itemID.String(), nil))
	assertContains(t, rec.Body.String(), `name="lock_version" value="0"`)

	rec = httptest.NewRecorder()
	f.handler.Update(rec, f.requestForItem(http.MethodPost, path, itemID.String(), url.Values{"lock_version": {"0"}, "quantity": {"2"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("先の更新 = %d、303を期待", rec.Code)
	}

	rec = httptest.NewRecorder()
	f.handler.Delete(rec, f.requestForItem(http.MethodPost, path, itemID.String(), url.Values{"lock_version": {"0"}}))
	if rec.Code != http.StatusConflict {
		t.Fatalf("古い画面からの削除 = %d、409を期待", rec.Code)
	}
	assertContains(t, rec.Body.String(), "先に変更されたため、操作しませんでした", `value="2"`, `name="lock_version" value="1"`)
	if item := f.findItem(t, itemID); item.Status != model.ItemStatusListed || item.Quantity != 2 || item.LockVersion != 1 {
		t.Errorf("アイテム = %+v、先の変更と版1を維持することを期待", item)
	}
}
