package admin_event_archive_test

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
	"github.com/cutreapp/cutre/go/internal/handler/admin_event_archive"
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

// fixture はテスト用のトランザクションの中で動く Handler と、そのトランザクションとイベントのリポジトリ。
type fixture struct {
	handler   *admin_event_archive.Handler
	tx        *sql.Tx
	eventRepo *repository.EventRepository
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	eventRepo := repository.NewEventRepository(db).WithTx(tx)
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	return &fixture{
		handler: admin_event_archive.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetAdminEventUsecase(eventRepo, repository.NewEventCategoryRepository(db).WithTx(tx)),
			usecase.NewArchiveEventUsecase(validator.NewEventArchiveCreateValidator(), eventRepo),
			usecase.NewUnarchiveEventUsecase(eventRepo),
		),
		tx:        tx,
		eventRepo: eventRepo,
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

// request は user がログインし、URLの {id} に eventID を載せたリクエストを作る。form があればフォームの本文にする。
func request(method string, user *model.User, eventID model.EventID, form url.Values) *http.Request {
	target := "/admin/events/" + eventID.String() + "/archive"
	var req *http.Request
	if form == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", eventID.String())
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, user)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// unarchive は user として、アーカイブしたイベントを公開に戻すフォームを版 lockVersion で送る。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
// GoはDELETEの本文をフォームとして読まないため、版はPOSTのうちに MethodOverride が読んだものをハンドラーが使う。
func (f *fixture) unarchive(user *model.User, eventID model.EventID, lockVersion string) *httptest.ResponseRecorder {
	form := url.Values{"_method": {http.MethodDelete}, "lock_version": {lockVersion}}
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(f.handler.Delete)).ServeHTTP(rec, request(http.MethodPost, user, eventID, form))

	return rec
}

// TestNew は、編集者に理由の入力欄を持つアーカイブの画面を描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.tx).WithName("秋のくじ").Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, f.user(t, model.UserRoleEditor), id, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<title>イベントをアーカイブ | Cutre</title>",
		`href="/admin/events/` + id.String() + `/edit"`,
		"「秋のくじ」をアーカイブします。",
		`action="/admin/events/` + id.String() + `/archive" method="post"`,
		`<textarea id="archive_message" name="archive_message"`,
		`name="lock_version" value="0"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// TestCreate_Conflict は、アーカイブの画面を開いたあとに変更されたイベントを古い版からアーカイブしないことを検証する。
func TestCreate_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.tx).WithName("変更前のくじ").Build()
	if updated, err := f.eventRepo.Update(t.Context(), id, 0, repository.EventAttributes{
		Name: "変更後のくじ", StartsOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {"古い理由"}, "lock_version": {"0"}}))
	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	for _, want := range []string{"変更後のくじ", "ほかの操作で先に更新されたため", "イベントを確認する"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	if found, _ := f.eventRepo.FindByID(t.Context(), id); found.IsArchived() {
		t.Error("古い画面からイベントをアーカイブした")
	}
}

// TestCreate_InvalidKeepsVersion は、入力を直す間に先の更新を見落とさないよう、422でも送った版を持ち回ることを検証する。
func TestCreate_InvalidKeepsVersion(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.tx).Build()
	if updated, err := f.eventRepo.Update(t.Context(), id, 0, repository.EventAttributes{
		Name: "変更後のくじ", StartsOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {""}, "lock_version": {"0"}}))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), `name="lock_version" value="0"`) {
		t.Error("入力エラーの描き直しで古い版が維持されていない")
	}
}

// TestDelete_Conflict は、公開に戻す画面を開いたあとに変更されたイベントを古い版から戻さないことを検証する。
func TestDelete_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.tx).WithArchived("終わったため").Build()
	if updated, err := f.eventRepo.Update(t.Context(), id, 0, repository.EventAttributes{
		Name: "変更後のくじ", StartsOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := f.unarchive(f.user(t, model.UserRoleEditor), id, "0")
	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	for _, want := range []string{"変更後のくじ", "アーカイブ", "ほかの操作で先に更新されたため"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	if found, _ := f.eventRepo.FindByID(t.Context(), id); !found.IsArchived() {
		t.Error("古い画面からイベントを公開に戻した")
	}
}

// TestNew_Archived は、既にアーカイブしたイベントでは編集の画面へ送ることを検証する。
func TestNew_Archived(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.tx).WithArchived("終わったため").Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, f.user(t, model.UserRoleEditor), id, nil))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/events/"+id.String()+"/edit" {
		t.Errorf("応答 = %d %q、303 編集の画面を期待", rec.Code, rec.Header().Get("Location"))
	}
}

// TestNew_NotFound は、一般のユーザーと削除したイベントに404を返すことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	for name, tt := range map[string]struct {
		user    *model.User
		eventID model.EventID
	}{
		"一般のユーザー":  {user: f.user(t, model.UserRoleUser), eventID: testutil.NewEventBuilder(t, f.tx).Build()},
		"削除したイベント": {user: f.user(t, model.UserRoleEditor), eventID: testutil.NewEventBuilder(t, f.tx).WithDeleted().Build()},
	} {
		rec := httptest.NewRecorder()
		f.handler.New(rec, request(http.MethodGet, tt.user, tt.eventID, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestCreate は、理由を残してイベントをアーカイブし、編集の画面へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.tx).Build()

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {"開催が終わったため"}, "lock_version": {"0"}}))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/events/"+id.String()+"/edit" {
		t.Fatalf("応答 = %d %q、303 編集の画面を期待", rec.Code, rec.Header().Get("Location"))
	}
	found, _ := f.eventRepo.FindByID(context.Background(), id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "開催が終わったため" {
		t.Errorf("アーカイブしたイベント = %+v、理由を残したアーカイブを期待", found)
	}
}

// TestCreate_Invalid は、理由が空のときにアーカイブせず、エラーを付けて描き直す (422) ことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.tx).Build()

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {" "}}))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), "入力してください") {
		t.Error("レスポンスボディに理由のエラーが含まれていない")
	}
	if found, _ := f.eventRepo.FindByID(context.Background(), id); found.IsArchived() {
		t.Error("理由が空なのにアーカイブした")
	}
}

// TestDelete は、アーカイブしたイベントを公開に戻して編集の画面へ戻し、一般のユーザーには404を返すことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewEventBuilder(t, f.tx).WithArchived("終わったため").Build()

	rec := f.unarchive(f.user(t, model.UserRoleUser), id, "0")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("一般のユーザーのステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}

	rec = f.unarchive(f.user(t, model.UserRoleEditor), id, "0")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/events/"+id.String()+"/edit" {
		t.Fatalf("応答 = %d %q、303 編集の画面を期待", rec.Code, rec.Header().Get("Location"))
	}
	if found, _ := f.eventRepo.FindByID(context.Background(), id); found.Status != model.MasterStatusPublished {
		t.Errorf("状態 = %q、公開中を期待", found.Status)
	}
}
