package admin_station_archive_test

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
	"github.com/cutreapp/cutre/go/internal/handler/admin_station_archive"
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

// fixture はテスト用のトランザクションの中で動く Handler と、そのトランザクションと駅のリポジトリ。
type fixture struct {
	handler     *admin_station_archive.Handler
	tx          *sql.Tx
	stationRepo *repository.StationRepository
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	stationRepo := repository.NewStationRepository(db).WithTx(tx)
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	return &fixture{
		handler: admin_station_archive.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetAdminStationUsecase(stationRepo),
			usecase.NewArchiveStationUsecase(validator.NewStationArchiveCreateValidator(), stationRepo),
			usecase.NewUnarchiveStationUsecase(stationRepo),
		),
		tx:          tx,
		stationRepo: stationRepo,
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

// station はテスト用の駅を作る StationBuilder を返す。
func (f *fixture) station(t *testing.T) *testutil.StationBuilder {
	t.Helper()

	return testutil.NewStationBuilder(t, f.tx)
}

// request は user がログインし、URLの {id} に id を載せたリクエストを作る。form があればフォームの本文にする。
func request(method string, user *model.User, id model.StationID, form url.Values) *http.Request {
	target := "/admin/stations/" + id.String() + "/archive"
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

// unarchive は user として、アーカイブした駅を公開に戻すフォームを版 lockVersion で送る。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
func (f *fixture) unarchive(user *model.User, id model.StationID, lockVersion string) *httptest.ResponseRecorder {
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

// TestNew は、編集者に、駅の一覧と駅へのパンくずと理由の入力欄を持つアーカイブの画面を描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.station(t).WithName("新宿").Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, f.user(t, model.UserRoleEditor), id, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"<title>駅をアーカイブ | Cutre</title>",
		`href="/admin/stations"`,
		`href="/admin/stations/`+id.String()+`/edit"`,
		"「新宿」をアーカイブします。",
		`action="/admin/stations/`+id.String()+`/archive" method="post"`,
		`<textarea id="archive_message" name="archive_message"`,
		`name="lock_version" value="0"`,
	)
}

// TestNew_Archived は、既にアーカイブした駅では編集の画面へ送ることを検証する。
func TestNew_Archived(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.station(t).WithArchived("閉業したため").Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, f.user(t, model.UserRoleEditor), id, nil))

	assertRedirect(t, rec, "/admin/stations/"+id.String()+"/edit")
}

// TestNew_NotFound は、一般のユーザーと削除した駅に404を返すことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	for name, tt := range map[string]struct {
		user *model.User
		id   model.StationID
	}{
		"一般のユーザー": {user: f.user(t, model.UserRoleUser), id: f.station(t).Build()},
		"削除した駅":   {user: f.user(t, model.UserRoleEditor), id: f.station(t).WithDeleted().Build()},
	} {
		rec := httptest.NewRecorder()
		f.handler.New(rec, request(http.MethodGet, tt.user, tt.id, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestCreate は、理由を残して駅をアーカイブし、編集の画面へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.station(t).Build()

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {"閉業したため"}, "lock_version": {"0"}}))

	assertRedirect(t, rec, "/admin/stations/"+id.String()+"/edit")
	found, _ := f.stationRepo.FindByID(t.Context(), id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "閉業したため" {
		t.Errorf("アーカイブした駅 = %+v、理由を残したアーカイブを期待", found)
	}
}

// TestCreate_Invalid は、理由が空のときにアーカイブせず、送った版を持ち回ってエラーを付けて描き直す (422) ことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.station(t).Build()
	if updated, err := f.stationRepo.Update(t.Context(), id, 0, repository.StationAttributes{PrefectureCode: 13, Name: "変更後の駅", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {" "}, "lock_version": {"0"}}))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), "入力してください", `name="lock_version" value="0"`)
	if found, _ := f.stationRepo.FindByID(t.Context(), id); found.IsArchived() {
		t.Error("理由が空なのにアーカイブした")
	}
}

// TestCreate_Conflict は、アーカイブの画面を開いたあとに変更された駅を古い版からアーカイブしないことを検証する。
func TestCreate_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.station(t).Build()
	if updated, err := f.stationRepo.Update(t.Context(), id, 0, repository.StationAttributes{PrefectureCode: 13, Name: "変更後の駅", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, f.user(t, model.UserRoleEditor), id, url.Values{"archive_message": {"古い理由"}, "lock_version": {"0"}}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	assertContains(t, rec.Body.String(), "変更後の駅", "ほかの操作で先に更新されたため", "駅を確認する")
	if found, _ := f.stationRepo.FindByID(t.Context(), id); found.IsArchived() {
		t.Error("古い画面から駅をアーカイブした")
	}
}

// TestDelete は、アーカイブした駅を公開に戻して編集の画面へ戻し、一般のユーザーには404を返すことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.station(t).WithArchived("閉業したため").Build()

	if rec := f.unarchive(f.user(t, model.UserRoleUser), id, "0"); rec.Code != http.StatusNotFound {
		t.Fatalf("一般のユーザーのステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}

	assertRedirect(t, f.unarchive(f.user(t, model.UserRoleEditor), id, "0"), "/admin/stations/"+id.String()+"/edit")
	if found, _ := f.stationRepo.FindByID(t.Context(), id); found.Status != model.MasterStatusPublished {
		t.Errorf("状態 = %q、公開中を期待", found.Status)
	}
}

// TestDelete_Conflict は、公開に戻す画面を開いたあとに変更された駅を古い版から戻さないことを検証する。
func TestDelete_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.station(t).WithArchived("閉業したため").Build()
	if updated, err := f.stationRepo.Update(t.Context(), id, 0, repository.StationAttributes{PrefectureCode: 13, Name: "変更後の駅", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := f.unarchive(f.user(t, model.UserRoleEditor), id, "0")
	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	assertContains(t, rec.Body.String(), "変更後の駅", "アーカイブ", "ほかの操作で先に更新されたため")
	if found, _ := f.stationRepo.FindByID(t.Context(), id); !found.IsArchived() {
		t.Error("古い画面から駅を公開に戻した")
	}
}
