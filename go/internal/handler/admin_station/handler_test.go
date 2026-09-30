package admin_station_test

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
	"github.com/cutreapp/cutre/go/internal/handler/admin_station"
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

// fixture は Handler と、テストデータを書き込む接続・駅のリポジトリ。
type fixture struct {
	handler     *admin_station.Handler
	conn        dbConn
	stationRepo *repository.StationRepository
	userRepo    *repository.UserRepository
}

// dbConn はテストデータを書き込む接続。テスト用のトランザクションか、共有の接続プールのどちらか。
type dbConn interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// newFixture はテスト用のトランザクションの中で動く fixture を返す。書いた行はテストの終了時にロールバックされる。
// 削除のUseCaseは自分でトランザクションを開き、このトランザクションで書いた行を読めないため、削除を送るテストでは newCommittedFixture を使う。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	return buildFixture(db, tx, repository.NewStationRepository(db).WithTx(tx), repository.NewUserStationRepository(db).WithTx(tx), repository.NewUserRepository(db).WithTx(tx))
}

// newCommittedFixture はテスト用のデータベースに直接書き込む fixture を返す (テストデータはコミットされる)。
// 自分でトランザクションを開く削除のUseCaseを通すテストで使う。
func newCommittedFixture(t *testing.T) *fixture {
	t.Helper()

	db := testutil.GetTestDB()
	return buildFixture(db, db, repository.NewStationRepository(db), repository.NewUserStationRepository(db), repository.NewUserRepository(db))
}

// buildFixture はリポジトリを使う Handler を組み立てる。削除のUseCaseには、自分でトランザクションを開くための db を渡す。
func buildFixture(db *sql.DB, conn dbConn, stationRepo *repository.StationRepository, userStationRepo *repository.UserStationRepository, userRepo *repository.UserRepository) *fixture {
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	return &fixture{
		handler: admin_station.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetAdminStationsUsecase(stationRepo),
			usecase.NewGetAdminStationUsecase(stationRepo),
			usecase.NewCreateStationUsecase(validator.NewStationCreateValidator(), stationRepo),
			usecase.NewUpdateStationUsecase(validator.NewStationUpdateValidator(), stationRepo),
			usecase.NewDeleteStationUsecase(db, validator.NewStationDeleteValidator(userStationRepo), stationRepo),
		),
		conn:        conn,
		stationRepo: stationRepo,
		userRepo:    userRepo,
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

// deleteStation は user として、駅の削除のフォームを版 lockVersion で送る。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
func (f *fixture) deleteStation(user *model.User, id model.StationID, lockVersion string) *httptest.ResponseRecorder {
	form := url.Values{"_method": {http.MethodDelete}, "lock_version": {lockVersion}}
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(f.handler.Delete)).ServeHTTP(rec, request(http.MethodPost, "/admin/stations/"+id.String(), user, id.String(), form))

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

// TestIndex は、編集者に、駅を都道府県ごとにまとめた一覧と、都道府県を選んだ作成の画面へのリンクを描画し、
// 削除した駅は出さないことを検証する。
func TestIndex(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shinjukuID := testutil.NewStationBuilder(t, f.conn).WithPrefectureCode(13).WithName("テストの新宿").Build()
	testutil.NewStationBuilder(t, f.conn).WithPrefectureCode(27).WithName("テストの梅田").WithArchived("閉業したため").Build()
	testutil.NewStationBuilder(t, f.conn).WithPrefectureCode(13).WithName("削除したテストの駅").WithDeleted().Build()

	rec := httptest.NewRecorder()
	f.handler.Index(rec, request(http.MethodGet, "/admin/stations", f.user(t, model.UserRoleEditor), "", nil))

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body,
		"<title>駅 | Cutre</title>",
		`href="/admin/stations/new"`,
		`<h2 id="prefecture-13-heading" class="px-4 font-semibold md:px-0">東京都</h2>`,
		`<h2 id="prefecture-27-heading" class="px-4 font-semibold md:px-0">大阪府</h2>`,
		`href="/admin/stations/`+shinjukuID.String()+`/edit"`,
		"テストの新宿",
		"テストの梅田",
		`href="/admin/stations/new?prefecture_code=13"`,
		"東京都に駅を作成",
	)
	if strings.Contains(body, "削除したテストの駅") {
		t.Error("削除した駅を一覧に出している")
	}
}

// TestIndex_NotFound は、一般のユーザーに404を返すことを検証する。
func TestIndex_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.Index(rec, request(http.MethodGet, "/admin/stations", f.user(t, model.UserRoleUser), "", nil))

	assertStatus(t, rec, http.StatusNotFound)
}

// TestNew は、クエリの都道府県を選び、その都道府県の既存の駅の最後の並び順に100を足した値を入れた作成のフォームを描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	// 並行するほかのテストがコミットした駅と重ならないよう、大きな並び順を使う。
	testutil.NewStationBuilder(t, f.conn).WithPrefectureCode(47).WithPosition(9000).Build()

	rec := httptest.NewRecorder()
	f.handler.New(rec, request(http.MethodGet, "/admin/stations/new?prefecture_code=47", f.user(t, model.UserRoleEditor), "", nil))

	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec.Body.String(),
		"<title>駅を作成 | Cutre</title>",
		`action="/admin/stations" method="post"`,
		`<option value="47" selected>沖縄県</option>`,
		`value="9100"`,
	)
}

// TestNew_WithoutPrefecture は、都道府県を受け取らなかったときと読めない値のときに、
// 都道府県を選ばず並び順を100にした作成のフォームを描画することを検証する。
func TestNew_WithoutPrefecture(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	editor := f.user(t, model.UserRoleEditor)
	for _, target := range []string{"/admin/stations/new", "/admin/stations/new?prefecture_code=99"} {
		rec := httptest.NewRecorder()
		f.handler.New(rec, request(http.MethodGet, target, editor, "", nil))

		assertStatus(t, rec, http.StatusOK)
		body := rec.Body.String()
		assertContains(t, body, `<option value="">選んでください</option>`, `value="100"`)
		if strings.Contains(body, " selected>") {
			t.Errorf("%s: 都道府県を選んでいる", target)
		}
	}
}

// TestCreate は、駅を作成し、完了のメッセージを付けて駅の一覧へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	form := url.Values{"prefecture_code": {"27"}, "name": {"テストの難波"}, "position": {"3"}}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/stations", f.user(t, model.UserRoleEditor), "", form))

	assertRedirect(t, rec, "/admin/stations")
	stations, err := f.stationRepo.ListUndeleted(t.Context())
	if err != nil {
		t.Fatalf("ListUndeleted()のエラー = %v", err)
	}
	var found bool
	for _, station := range stations {
		found = found || (station.Name == "テストの難波" && station.PrefectureCode == 27 && station.Position == 3)
	}
	if !found {
		t.Error("大阪府 (27) の「テストの難波」・並び順3が作成されていない")
	}
}

// TestCreate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直す (422) ことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	form := url.Values{"prefecture_code": {""}, "name": {"テストの難波"}, "position": {"2"}}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, request(http.MethodPost, "/admin/stations", f.user(t, model.UserRoleEditor), "", form))

	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), "選んでください", `value="テストの難波"`, `value="2"`, `aria-describedby="prefecture_code-error-0"`)
}

// TestEdit は、編集者に、保存済みの値・版を入れた編集のフォームとアーカイブの画面へのリンクを描画し、
// 削除の欄は出さないことを検証する。
func TestEdit(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewStationBuilder(t, f.conn).WithPrefectureCode(14).WithName("横浜").WithPosition(3).Build()

	rec := httptest.NewRecorder()
	f.handler.Edit(rec, request(http.MethodGet, "/admin/stations/"+id.String()+"/edit", f.user(t, model.UserRoleEditor), id.String(), nil))

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body,
		`<h1 class="text-xl font-semibold">横浜</h1>`,
		`href="/admin/stations"`,
		`action="/admin/stations/`+id.String()+`" method="post"`,
		`<input type="hidden" name="lock_version" value="0">`,
		`<option value="14" selected>神奈川県</option>`,
		`value="3"`,
		`href="/admin/stations/`+id.String()+`/archive/new"`,
	)
	if strings.Contains(body, "station-delete-dialog") {
		t.Error("編集者に削除の欄を出している")
	}
}

// TestEdit_Admin は、管理者には削除の欄を出し、アーカイブした駅には理由と元に戻すフォームを出すことを検証する。
func TestEdit_Admin(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewStationBuilder(t, f.conn).WithName("渋谷").WithArchived("閉業したため").Build()

	rec := httptest.NewRecorder()
	f.handler.Edit(rec, request(http.MethodGet, "/admin/stations/"+id.String()+"/edit", f.user(t, model.UserRoleAdmin), id.String(), nil))

	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec.Body.String(),
		"閉業したため",
		`action="/admin/stations/`+id.String()+`/archive" method="post"`,
		">元に戻す<",
		`commandfor="station-delete-dialog" command="show-modal"`,
		"「渋谷」を削除します。元に戻せません。",
	)
}

// TestEdit_NotFound は、一般のユーザー・削除した駅・UUIDとして読めないIDに404を返すことを検証する。
func TestEdit_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	editor := f.user(t, model.UserRoleEditor)
	for name, tt := range map[string]struct {
		user *model.User
		id   string
	}{
		"一般のユーザー": {user: f.user(t, model.UserRoleUser), id: testutil.NewStationBuilder(t, f.conn).Build().String()},
		"削除した駅":   {user: editor, id: testutil.NewStationBuilder(t, f.conn).WithDeleted().Build().String()},
		"読めないID":  {user: editor, id: "not-a-uuid"},
	} {
		rec := httptest.NewRecorder()
		f.handler.Edit(rec, request(http.MethodGet, "/admin/stations/"+tt.id+"/edit", tt.user, tt.id, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestUpdate は、駅を更新し、完了のメッセージを付けて駅の一覧へ戻すことを検証する。
func TestUpdate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewStationBuilder(t, f.conn).Build()
	form := url.Values{"lock_version": {"0"}, "prefecture_code": {"23"}, "name": {"名古屋"}, "position": {"7"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/stations/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertRedirect(t, rec, "/admin/stations")
	found, _ := f.stationRepo.FindByID(t.Context(), id)
	if found.PrefectureCode != 23 || found.Name != "名古屋" || found.Position != 7 || found.LockVersion != 1 {
		t.Errorf("更新した駅 = %+v、愛知県 (23) の「名古屋」・並び順7・版1を期待", found)
	}
}

// TestUpdate_Conflict は、フォームを開いたあとにほかの操作で先に更新されていたら上書きせず、
// 最新の値と版で編集の画面を描き直す (409) ことを検証する。
func TestUpdate_Conflict(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewStationBuilder(t, f.conn).Build()
	if updated, err := f.stationRepo.Update(t.Context(), id, 0, repository.StationAttributes{PrefectureCode: 13, Name: "先に更新された駅", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}
	form := url.Values{"lock_version": {"0"}, "prefecture_code": {"13"}, "name": {"古い版からの駅"}, "position": {"2"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/stations/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertStatus(t, rec, http.StatusConflict)
	body := rec.Body.String()
	assertContains(t, body, "ほかの操作で先に更新されたため、保存しませんでした", `value="先に更新された駅"`, `name="lock_version" value="1"`)
	if strings.Contains(body, "古い版からの駅") {
		t.Error("競合した送信の値をフォームに戻している")
	}
}

// TestUpdate_Invalid は、フォームの誤りを、送られた値とエラーを付けて描き直す (422) ことを検証する。
func TestUpdate_Invalid(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := testutil.NewStationBuilder(t, f.conn).Build()
	form := url.Values{"lock_version": {"0"}, "prefecture_code": {"13"}, "name": {"新宿"}, "position": {"x"}}

	rec := httptest.NewRecorder()
	f.handler.Update(rec, request(http.MethodPatch, "/admin/stations/"+id.String(), f.user(t, model.UserRoleEditor), id.String(), form))

	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), "0から9999までの整数を入力してください", `value="x"`)
}

// TestDelete は、管理者が駅を削除して駅の一覧へ戻れ、編集者には404を返して削除しないことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	id := testutil.NewStationBuilder(t, f.conn).Build()

	assertStatus(t, f.deleteStation(f.user(t, model.UserRoleEditor), id, "0"), http.StatusNotFound)

	assertRedirect(t, f.deleteStation(f.user(t, model.UserRoleAdmin), id, "0"), "/admin/stations")
	if found, _ := f.stationRepo.FindByID(t.Context(), id); !found.IsDeleted() {
		t.Errorf("状態 = %q、削除した状態を期待", found.Status)
	}
}

// TestDelete_Conflict は、確認の画面を開いたあとに変更された駅を古い版から削除せず、最新の値で描き直す (409) ことを検証する。
func TestDelete_Conflict(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	id := testutil.NewStationBuilder(t, f.conn).Build()
	if updated, err := f.stationRepo.Update(t.Context(), id, 0, repository.StationAttributes{PrefectureCode: 13, Name: "変更後の駅", Position: 1}); err != nil || !updated {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	rec := f.deleteStation(f.user(t, model.UserRoleAdmin), id, "0")

	assertStatus(t, rec, http.StatusConflict)
	assertContains(t, rec.Body.String(), "変更後の駅", "ほかの操作で先に更新されたため", `name="lock_version" value="1"`)
	if found, _ := f.stationRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("古い画面から駅を削除した")
	}
}

// TestDelete_Referenced は、交換場所に選ばれている駅を削除せず、
// アーカイブを案内するエラーを付けて編集の画面を描き直すことを検証する。
func TestDelete_Referenced(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	admin := f.user(t, model.UserRoleAdmin)
	id := testutil.NewStationBuilder(t, f.conn).Build()
	testutil.NewUserStationBuilder(t, f.conn, admin.ID, id).Build()

	rec := f.deleteStation(admin, id, "0")
	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), "この駅は交換場所に選ばれているため、削除できません。代わりにアーカイブしてください")
	if found, _ := f.stationRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("交換場所に選ばれている駅を削除した")
	}
}
