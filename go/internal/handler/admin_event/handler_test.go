package admin_event_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/admin_event"
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

// fixture は Handler と、テストデータを書き込む接続・イベントのリポジトリ。
type fixture struct {
	handler   *admin_event.Handler
	conn      dbConn
	eventRepo *repository.EventRepository
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
		item:     repository.NewItemRepository(db),
		user:     repository.NewUserRepository(db),
	})
}

// buildFixture は repos を使う Handler を組み立てる。削除のUseCaseには、自分でトランザクションを開くための db を渡す。
func buildFixture(db *sql.DB, conn dbConn, repos fixtureRepos) *fixture {
	eventRepo, categoryRepo, itemRepo := repos.event, repos.category, repos.item
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	return &fixture{
		handler: admin_event.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetAdminMenuUsecase(),
			usecase.NewGetAdminEventsUsecase(eventRepo),
			usecase.NewGetAdminEventUsecase(eventRepo, categoryRepo),
			usecase.NewCreateEventUsecase(validator.NewEventCreateValidator(), eventRepo),
			usecase.NewUpdateEventUsecase(validator.NewEventUpdateValidator(), eventRepo),
			usecase.NewDeleteEventUsecase(db, validator.NewEventDeleteValidator(itemRepo), eventRepo),
		),
		conn:      conn,
		eventRepo: eventRepo,
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

// request は user がログインしたリクエストを作る。eventID が空でなければURLの {id} に載せ、form があればフォームの本文にする。
func request(method, target string, user *model.User, eventID string, form url.Values) *http.Request {
	var req *http.Request
	if form == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, user)
	if eventID != "" {
		routeCtx := chi.NewRouteContext()
		routeCtx.URLParams.Add("id", eventID)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
	}

	return req.WithContext(ctx)
}

// deleteEvent は user として、イベントの削除のフォームを版 lockVersion で送る。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
// GoはDELETEの本文をフォームとして読まないため、版はPOSTのうちに MethodOverride が読んだものをハンドラーが使う。
func (f *fixture) deleteEvent(user *model.User, id model.EventID, lockVersion string) *httptest.ResponseRecorder {
	form := url.Values{"_method": {http.MethodDelete}, "lock_version": {lockVersion}}
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(f.handler.Delete)).ServeHTTP(rec, request(http.MethodPost, "/admin/events/"+id.String(), user, id.String(), form))

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

// assertNotContains はボディに absents のいずれも含まれないことを検証する。
func assertNotContains(t *testing.T, body string, absents ...string) {
	t.Helper()

	for _, absent := range absents {
		if strings.Contains(body, absent) {
			t.Errorf("レスポンスボディに %q が含まれている", absent)
		}
	}
}

// assertFormLockVersion は送信先と上書きメソッドで選んだフォームが、期待する版を持つことを検証する。
func assertFormLockVersion(t *testing.T, body, action, method, version string) {
	t.Helper()

	for _, section := range strings.Split(body, "</form>") {
		start := strings.LastIndex(section, "<form")
		if start < 0 {
			continue
		}
		form := section[start:]
		if strings.Contains(form, `action="`+action+`"`) && strings.Contains(form, `name="_method" value="`+method+`"`) {
			if !strings.Contains(form, `name="lock_version" value="`+version+`"`) {
				t.Errorf("%s %s のフォームに版 %s が無い", method, action, version)
			}
			return
		}
	}

	t.Errorf("%s %s のフォームが無い", method, action)
}

// assertRedirect は応答が location への303であることを検証する。
func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, location string) {
	t.Helper()

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != location {
		t.Fatalf("応答 = %d %q、303 %q を期待", rec.Code, rec.Header().Get("Location"), location)
	}
}

// TestIndex は、編集者にイベントの一覧を描画し、アーカイブしたイベントにバッジを付け、削除したイベントを出さないことを検証する。
func TestIndex(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	endsOn := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	publishedID := testutil.NewEventBuilder(t, f.conn).WithName("公開中のくじ").WithPeriod(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), &endsOn).Build()
	testutil.NewEventBuilder(t, f.conn).WithName("アーカイブしたくじ").WithArchived("終わったため").Build()
	testutil.NewEventBuilder(t, f.conn).WithName("削除したくじ").WithDeleted().Build()

	rec := httptest.NewRecorder()
	f.handler.Index(rec, request(http.MethodGet, "/admin/events", f.user(t, model.UserRoleEditor), "", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>イベントの管理 | Cutre</title>",
		`href="/admin/events/new"`,
		`href="/admin/events/`+publishedID.String()+`/edit"`,
		"公開中のくじ",
		"2026年10月1日〜2026年10月31日",
		"アーカイブしたくじ",
		`<span class="badge" data-variant="warning">アーカイブ</span>`,
	)
	assertNotContains(t, body, "削除したくじ")
}

// TestIndex_NotFound は、一般のユーザーには一覧を出さず404を返すことを検証する。
func TestIndex_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.Index(rec, request(http.MethodGet, "/admin/events", f.user(t, model.UserRoleUser), "", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestNew は、編集者に作成のフォームを描画し、一般のユーザーには404を返すことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, "/admin/events/new", f.user(t, model.UserRoleEditor), "", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		`action="/admin/events" method="post"`,
		`name="name"`,
		`name="starts_on" type="date"`,
		`name="ends_on" type="date"`,
		">作成する<",
	)

	rec = httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, "/admin/events/new", f.user(t, model.UserRoleUser), "", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("一般のユーザーのステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestCreate は、イベントを作成し、完了のメッセージを付けて一覧へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	form := url.Values{"name": {"作成したくじ"}, "starts_on": {"2026-10-01"}, "ends_on": {""}}
	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/events", f.user(t, model.UserRoleEditor), "", form))

	assertRedirect(t, rec, "/admin/events")
	events, err := f.eventRepo.ListUndeleted(context.Background())
	if err != nil {
		t.Fatalf("ListUndeleted()のエラー = %v", err)
	}
	found := false
	for _, event := range events {
		found = found || event.Name == "作成したくじ"
	}
	if !found {
		t.Error("作成したイベントが一覧に無い")
	}
}

// TestCreate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直す (422) ことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	form := url.Values{"name": {"くじ"}, "starts_on": {"2026-10-01"}, "ends_on": {"2026-09-30"}}
	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/events", f.user(t, model.UserRoleEditor), "", form))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), `value="くじ"`, `value="2026-09-30"`, "開始日以降の日付を入力してください")
}

// TestCreate_NotFound は、一般のユーザーの送信を受け付けず404を返すことを検証する。
func TestCreate_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	form := url.Values{"name": {"くじ"}, "starts_on": {"2026-10-01"}}
	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/events", f.user(t, model.UserRoleUser), "", form))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestEdit は、編集者に保存済みの値と版を入れた編集のフォーム・カテゴリーの一覧・アーカイブの画面へのリンクを描画し、
// 削除したカテゴリーと削除の欄は出さないことを検証する。
func TestEdit(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.conn).WithName("編集するくじ").Build()
	categoryID := testutil.NewEventCategoryBuilder(t, f.conn, id).WithName("A賞").Build()
	testutil.NewEventCategoryBuilder(t, f.conn, id).WithName("削除したカテゴリー").WithDeleted().Build()

	rec := httptest.NewRecorder()
	f.handler.Edit(rec, request(http.MethodGet, "/admin/events/"+id.String()+"/edit", f.user(t, model.UserRoleEditor), id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`<h1 class="text-xl font-semibold">編集するくじ</h1>`,
		`action="/admin/events/`+id.String()+`" method="post"`,
		`<input type="hidden" name="_method" value="PATCH">`,
		`<input type="hidden" name="lock_version" value="0">`,
		`value="2026-09-01"`,
		`href="/admin/events/`+id.String()+`/archive/new"`,
		`href="/admin/categories/`+categoryID.String()+`/edit"`,
		"A賞",
		`href="/admin/events/`+id.String()+`/categories/new"`,
	)
	assertNotContains(t, body, "event-delete-dialog", "削除したカテゴリー")
}

// TestEdit_Admin は、管理者には削除の欄と確認のダイアログを出し、アーカイブしたイベントには理由と元に戻すフォームを出すことを検証する。
func TestEdit_Admin(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.conn).WithName("終わったくじ").WithArchived("開催が終わったため").Build()

	rec := httptest.NewRecorder()
	f.handler.Edit(rec, request(http.MethodGet, "/admin/events/"+id.String()+"/edit", f.user(t, model.UserRoleAdmin), id.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"開催が終わったため",
		`action="/admin/events/`+id.String()+`/archive" method="post"`,
		">元に戻す<",
		`commandfor="event-delete-dialog" command="show-modal"`,
		`name="lock_version" value="0"`,
		"「終わったくじ」を削除します。元に戻せません。",
	)
}

// TestEdit_NotFound は、一般のユーザー・削除したイベント・UUIDとして読めないIDに404を返すことを検証する。
func TestEdit_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	editor := f.user(t, model.UserRoleEditor)
	publishedID := testutil.NewEventBuilder(t, f.conn).Build().String()
	deletedID := testutil.NewEventBuilder(t, f.conn).WithDeleted().Build().String()

	for name, tt := range map[string]struct {
		user    *model.User
		eventID string
	}{
		"一般のユーザー":  {user: f.user(t, model.UserRoleUser), eventID: publishedID},
		"削除したイベント": {user: editor, eventID: deletedID},
		"読めないID":   {user: editor, eventID: "not-a-uuid"},
	} {
		rec := httptest.NewRecorder()
		f.handler.Edit(rec, request(http.MethodGet, "/admin/events/"+tt.eventID+"/edit", tt.user, tt.eventID, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestUpdate は、イベントを更新し、完了のメッセージを付けて一覧へ戻すことを検証する。
func TestUpdate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.conn).Build()
	form := url.Values{"lock_version": {"0"}, "name": {"更新したくじ"}, "starts_on": {"2026-11-01"}, "ends_on": {"2026-11-30"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/events/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertRedirect(t, rec, "/admin/events")
	found, _ := f.eventRepo.FindByID(context.Background(), id)
	if found.Name != "更新したくじ" || found.LockVersion != 1 {
		t.Errorf("更新したイベント = %+v、「更新したくじ」・版1を期待", found)
	}
}

// TestUpdate_Conflict は、フォームを開いたあとにほかの操作で先に更新されていたら上書きせず、
// 最新の値と版で編集の画面を描き直す (409) ことを検証する。
func TestUpdate_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.conn).WithName("先に更新されたくじ").Build()
	if _, err := f.eventRepo.Update(context.Background(), id, 0, repository.EventAttributes{Name: "先に更新されたくじ", StartsOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("先の更新のエラー = %v", err)
	}
	form := url.Values{"lock_version": {"0"}, "name": {"古い版からのくじ"}, "starts_on": {"2026-11-01"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/events/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	body := rec.Body.String()
	assertContains(t, body, "ほかの操作で先に更新されたため、保存しませんでした", `value="先に更新されたくじ"`, `name="lock_version" value="1"`)
	assertNotContains(t, body, "古い版からのくじ")
}

// TestUpdate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直す (422) ことを検証する。
func TestUpdate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.conn).Build()
	form := url.Values{"lock_version": {"0"}, "name": {""}, "starts_on": {"2026-11-01"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/events/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), "入力してください", `value="2026-11-01"`)
}

// TestUpdate_InvalidAfterConcurrentChange は入力エラーの画面で、編集フォームに送信時の版を残し、
// 保存済みのイベントに対する削除と公開復帰のフォームには最新の版を渡すことを検証する。
func TestUpdate_InvalidAfterConcurrentChange(t *testing.T) {
	t.Parallel()

	for _, archived := range []bool{false, true} {
		name := "公開中"
		if archived {
			name = "アーカイブ中"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newCommittedFixture(t)
			builder := testutil.NewEventBuilder(t, f.conn).WithName("変更前のくじ")
			if archived {
				builder = builder.WithArchived("終わったため")
			}
			id := builder.Build()
			if updated, err := f.eventRepo.Update(t.Context(), id, 0, repository.EventAttributes{
				Name: "変更後のくじ", StartsOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			}); err != nil || !updated {
				t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
			}

			admin := f.user(t, model.UserRoleAdmin)
			path := "/admin/events/" + id.String()
			form := url.Values{"lock_version": {"0"}, "name": {""}, "starts_on": {"2026-11-01"}}
			rec := httptest.NewRecorder()
			f.handler.Update(rec, request(http.MethodPatch, path, admin, id.String(), form))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
			}
			body := rec.Body.String()
			assertContains(t, body, "変更後のくじ", "入力してください")
			assertFormLockVersion(t, body, path, http.MethodPatch, "0")
			assertFormLockVersion(t, body, path, http.MethodDelete, "1")
			if archived {
				assertFormLockVersion(t, body, path+"/archive", http.MethodDelete, "1")
				if updated, err := f.eventRepo.Unarchive(t.Context(), id, 1); err != nil || !updated {
					t.Fatalf("画面の版で公開に戻す = (%v, %v)、成功を期待", updated, err)
				}
			} else {
				assertRedirect(t, f.deleteEvent(admin, id, "1"), "/admin/events")
			}
		})
	}
}

// TestDelete は、管理者がイベントを削除でき、編集者には404を返して削除しないことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	id := testutil.NewEventBuilder(t, f.conn).Build()

	rec := f.deleteEvent(f.user(t, model.UserRoleEditor), id, "0")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("編集者のステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}

	rec = f.deleteEvent(f.user(t, model.UserRoleAdmin), id, "0")
	assertRedirect(t, rec, "/admin/events")
	if found, _ := f.eventRepo.FindByID(context.Background(), id); !found.IsDeleted() {
		t.Errorf("状態 = %q、削除した状態を期待", found.Status)
	}
}

// TestDelete_Conflict は、確認の画面を開いたあとに変更されたイベントを古い版から削除しないことを検証する。
func TestDelete_Conflict(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	id := testutil.NewEventBuilder(t, f.conn).WithName("変更前のくじ").Build()
	if updated, err := f.eventRepo.Update(t.Context(), id, 0, repository.EventAttributes{
		Name: "変更後のくじ", StartsOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := f.deleteEvent(f.user(t, model.UserRoleAdmin), id, "0")
	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	assertContains(t, rec.Body.String(), "変更後のくじ", "ほかの操作で先に更新されたため", `name="lock_version" value="1"`)
	if found, _ := f.eventRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("古い画面からイベントを削除した")
	}
}
