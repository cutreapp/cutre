package settings_place_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/settings_place"
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

// fixture は Handler と、テストデータを書き込む接続・駅とユーザーのリポジトリ。
type fixture struct {
	handler     *settings_place.Handler
	conn        dbConn
	stationRepo *repository.StationRepository
	userRepo    *repository.UserRepository
}

// dbConn はテストデータを書き込む接続。テスト用のトランザクションか、共有の接続プールのどちらか。
type dbConn interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// newFixture はテスト用のトランザクションの中で動く fixture を返す。書いた行はテストの終了時にロールバックされる。
// 保存のUseCaseは自分でトランザクションを開き、このトランザクションで書いた行を読めないため、保存を送るテストでは newCommittedFixture を使う。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	return buildFixture(db, tx, repository.NewStationRepository(db).WithTx(tx), repository.NewUserStationRepository(db).WithTx(tx), repository.NewUserRepository(db).WithTx(tx))
}

// newCommittedFixture はテスト用のデータベースに直接書き込む fixture を返す (テストデータはコミットされる)。
func newCommittedFixture(t *testing.T) *fixture {
	t.Helper()

	db := testutil.GetTestDB()
	return buildFixture(db, db, repository.NewStationRepository(db), repository.NewUserStationRepository(db), repository.NewUserRepository(db))
}

// buildFixture はリポジトリを使う Handler を組み立てる。保存のUseCaseには、自分でトランザクションを開くための db を渡す。
func buildFixture(db *sql.DB, conn dbConn, stationRepo *repository.StationRepository, userStationRepo *repository.UserStationRepository, userRepo *repository.UserRepository) *fixture {
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	return &fixture{
		handler: settings_place.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			session.NewFlashManager(),
			usecase.NewGetPlacesUsecase(stationRepo, userRepo),
			usecase.NewGetPublishedStationsUsecase(stationRepo),
			usecase.NewUpdatePlacesUsecase(
				db, validator.NewPlaceUpdateValidator(), validator.NewPlaceStationUpdateValidator(stationRepo),
				stationRepo, userStationRepo, userRepo,
			),
		),
		conn:        conn,
		stationRepo: stationRepo,
		userRepo:    userRepo,
	}
}

// user はユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func (f *fixture) user(t *testing.T) *model.User {
	t.Helper()

	return f.reload(t, testutil.NewUserBuilder(t, f.conn).Build())
}

// reload はユーザー id を読み直す。
func (f *fixture) reload(t *testing.T, id model.UserID) *model.User {
	t.Helper()

	user, err := f.userRepo.FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// station は都道府県 code に名前 name の駅を作る。
func (f *fixture) station(t *testing.T, code model.PrefectureCode, name string) model.StationID {
	t.Helper()

	return testutil.NewStationBuilder(t, f.conn).WithPrefectureCode(code).WithName(name).Build()
}

// withUser はログイン中のユーザーと日本語のロケールを載せたリクエストを返す。user がnilのときはユーザーを載せない。
func withUser(req *http.Request, user *model.User) *http.Request {
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}

	return req.WithContext(ctx)
}

// show はログイン中のユーザーとして交換場所の画面を開く。
func (f *fixture) show(user *model.User) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	f.handler.Show(rec, withUser(httptest.NewRequest(http.MethodGet, "/settings/places", nil), user))

	return rec
}

// update はログイン中のユーザーとして、駅 stationIDs と「ほかに出られるところ」 placeNote で交換場所を保存する。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
func (f *fixture) update(user *model.User, stationIDs []string, placeNote string) *httptest.ResponseRecorder {
	version := int32(0)
	if user != nil {
		version = user.PlaceLockVersion
	}
	form := url.Values{"_method": {http.MethodPatch}, "station_ids": stationIDs, "place_note": {placeNote}, "lock_version": {strconv.FormatInt(int64(version), 10)}}
	req := httptest.NewRequest(http.MethodPost, "/settings/places", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(f.handler.Update)).ServeHTTP(rec, withUser(req, user))

	return rec
}

// selectedNames はユーザーが交換場所に選んだ駅の名前を並んだ順に返す。
func (f *fixture) selectedNames(t *testing.T, userID model.UserID) []string {
	t.Helper()

	stations, err := f.stationRepo.ListByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("交換場所の駅の取得に失敗: %v", err)
	}
	names := make([]string, len(stations))
	for i, station := range stations {
		names[i] = station.Name
	}

	return names
}

// checkbox は駅 id のチェックボックスのマークアップを返す。checked ならチェックを入れたものにする。
func checkbox(id model.StationID, checked bool) string {
	markup := `<input type="checkbox" name="station_ids" value="` + id.String() + `" class="sr-only"`
	if checked {
		return markup + " checked>"
	}

	return markup + ">"
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

// assertStatus は応答のステータスコードが want であることを検証する。
func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, want)
	}
}

// TestShow は、選んだ駅のある都道府県に、公開中の駅と選んだ駅 (アーカイブしたものを含む) をチェックボックスで並べ、
// 選んでいない都道府県を「都道府県を追加」の中に畳み、選んでいないアーカイブした駅は出さないことを検証する。
// あわせて、パンくず・「ほかに出られるところ」・PATCHに上書きする保存のフォームを描画することを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.user(t)
	shinjuku := f.station(t, 13, "新宿")
	shibuya := f.station(t, 13, "渋谷")
	archivedSelected := testutil.NewStationBuilder(t, f.conn).WithName("選んだあとに閉じた駅").WithArchived("閉業").Build()
	archived := testutil.NewStationBuilder(t, f.conn).WithName("閉じた駅").WithArchived("閉業").Build()
	yokohama := f.station(t, 14, "横浜")
	testutil.NewUserStationBuilder(t, f.conn, user.ID, shinjuku).Build()
	testutil.NewUserStationBuilder(t, f.conn, user.ID, archivedSelected).Build()
	if updated, err := f.userRepo.UpdatePlaces(t.Context(), user.ID, "平日の夜なら新宿の近く", user.PlaceLockVersion); err != nil || !updated {
		t.Fatalf("ほかに出られるところの保存に失敗: %v", err)
	}

	rec := f.show(f.reload(t, user.ID))

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body,
		"<title>交換場所 | Cutre</title>",
		`<meta name="robots" content="noindex">`,
		`<nav aria-label="パンくずリスト">`,
		`href="/@`+user.Atname+`"`,
		`<h1 class="text-xl font-semibold">交換場所</h1>`,
		`action="/settings/places" method="post"`,
		`<input type="hidden" name="_method" value="PATCH">`,
		`<input type="hidden" name="lock_version" value="1">`,
		`<legend class="mb-2.5 font-semibold">東京都</legend>`,
		checkbox(shinjuku, true),
		checkbox(shibuya, false),
		checkbox(archivedSelected, true),
		"都道府県を追加",
		checkbox(yokohama, false),
		`<legend class="sr-only">神奈川県</legend>`,
		`value="平日の夜なら新宿の近く"`,
		"保存する",
	)
	assertNotContains(t, body, archived.String(), `<details class="page-card group/add" open>`)
}

// TestShow_WithoutPlaces は、交換場所をまだ選んでいなければ、選ぶところから始められるよう「都道府県を追加」を開いておくことを検証する。
func TestShow_WithoutPlaces(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.user(t)
	shinjuku := f.station(t, 13, "新宿")

	rec := f.show(user)

	assertStatus(t, rec, http.StatusOK)
	assertContains(t, rec.Body.String(), `<details class="page-card group/add" open>`, checkbox(shinjuku, false))
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かの交換場所として描画しないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	assertStatus(t, newFixture(t).show(nil), http.StatusInternalServerError)
}

// TestUpdate は、選んだ駅と「ほかに出られるところ」を保存して、完了のメッセージを付けてマイページへ戻し、
// 次の保存でチェックを外した駅を交換場所から外すことを検証する。
func TestUpdate(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	user := f.user(t)
	shinjuku := f.station(t, 13, "新宿")
	kawasaki := f.station(t, 14, "川崎")

	rec := f.update(user, []string{kawasaki.String(), shinjuku.String(), shinjuku.String()}, "  平日の夜なら  ")

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/@"+user.Atname {
		t.Fatalf("応答 = %d %q、303 %q を期待", rec.Code, rec.Header().Get("Location"), "/@"+user.Atname)
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), session.FlashCookieName) {
		t.Error("完了のメッセージが設定されていない")
	}
	if got := f.selectedNames(t, user.ID); strings.Join(got, ",") != "新宿,川崎" {
		t.Errorf("交換場所の駅 = %v、新宿と川崎を期待", got)
	}
	if got := f.reload(t, user.ID).PlaceNote; got != "平日の夜なら" {
		t.Errorf("ほかに出られるところ = %q、前後の空白を除いた値を期待", got)
	}

	f.update(f.reload(t, user.ID), []string{kawasaki.String()}, "")
	if got := f.selectedNames(t, user.ID); strings.Join(got, ",") != "川崎" {
		t.Errorf("交換場所の駅 = %v、川崎だけを期待", got)
	}
	if got := f.reload(t, user.ID).PlaceNote; got != "" {
		t.Errorf("ほかに出られるところ = %q、空を期待", got)
	}
}

// TestUpdate_Conflict は、古い画面からの送信を409で拒否し、先に保存された駅とひとこと、最新の版を描き直すことを検証する。
func TestUpdate_Conflict(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	user := f.user(t)
	first := f.station(t, 13, "新宿")
	second := f.station(t, 14, "川崎")

	assertStatus(t, f.update(user, []string{first.String()}, "先に保存した内容"), http.StatusSeeOther)
	rec := f.update(user, []string{second.String()}, "古い画面の内容")

	assertStatus(t, rec, http.StatusConflict)
	assertContains(t, rec.Body.String(), "ほかの操作で交換場所が先に更新されました", checkbox(first, true), checkbox(second, false), `value="先に保存した内容"`, `name="lock_version" value="1"`)
	if got := f.selectedNames(t, user.ID); len(got) != 1 || got[0] != "新宿" {
		t.Errorf("交換場所 = %v、先に保存した駅だけを期待", got)
	}
	if got := f.reload(t, user.ID); got.PlaceNote != "先に保存した内容" || got.PlaceLockVersion != 1 {
		t.Errorf("ユーザー = %+v、先に保存したひとことと版1を期待", got)
	}
}

// TestShowAndUpdate_WithdrawnUser は、認証後に退会が完了したユーザーの画面と保存を404にすることを検証する。
func TestShowAndUpdate_WithdrawnUser(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	user := f.user(t)
	if withdrawn, err := f.userRepo.Withdraw(t.Context(), user.ID, model.AnonymizedEmail(user.ID), model.AnonymizedAtname(user.ID)); err != nil || !withdrawn {
		t.Fatalf("退会 = (%v, %v)、成功を期待", withdrawn, err)
	}

	assertStatus(t, f.show(user), http.StatusNotFound)
	assertStatus(t, f.update(user, nil, ""), http.StatusNotFound)
}

// TestUpdate_KeepsArchivedStation は、既に選んでいる駅はアーカイブしたあとも選んだままにでき、
// チェックを外せば交換場所から外せることを検証する。
func TestUpdate_KeepsArchivedStation(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	user := f.user(t)
	archived := testutil.NewStationBuilder(t, f.conn).WithName("閉じた駅").WithArchived("閉業").Build()
	testutil.NewUserStationBuilder(t, f.conn, user.ID, archived).Build()

	if rec := f.update(user, []string{archived.String()}, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if got := f.selectedNames(t, user.ID); len(got) != 1 {
		t.Errorf("交換場所の駅 = %v、アーカイブした駅を選んだままを期待", got)
	}

	if rec := f.update(f.reload(t, user.ID), nil, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if got := f.selectedNames(t, user.ID); len(got) != 0 {
		t.Errorf("交換場所の駅 = %v、空を期待", got)
	}
}

// TestUpdate_UnavailableStation は、選んでいない駅がアーカイブ・削除されていたり、駅のIDとして読めなかったりすれば保存せず、
// 選び直しを案内するエラーと送られた値で画面を描き直す (422) ことを検証する。
func TestUpdate_UnavailableStation(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	shinjuku := f.station(t, 13, "新宿")
	for name, unavailable := range map[string]string{
		"アーカイブした駅": testutil.NewStationBuilder(t, f.conn).WithArchived("閉業").Build().String(),
		"削除した駅":    testutil.NewStationBuilder(t, f.conn).WithDeleted().Build().String(),
		"読めないID":   "not-a-uuid",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			user := f.user(t)
			rec := f.update(user, []string{shinjuku.String(), unavailable}, "平日の夜なら")

			assertStatus(t, rec, http.StatusUnprocessableEntity)
			assertContains(t, rec.Body.String(), "選べなくなった駅があります。選び直してから、もう一度保存してください", checkbox(shinjuku, true), `value="平日の夜なら"`)
			if got := f.selectedNames(t, user.ID); len(got) != 0 {
				t.Errorf("交換場所の駅 = %v、保存しないことを期待", got)
			}
		})
	}
}

// TestUpdate_Invalid は、「ほかに出られるところ」が長すぎれば保存せず、送られた値とエラーで画面を描き直す (422) ことを検証する。
func TestUpdate_Invalid(t *testing.T) {
	t.Parallel()

	f := newCommittedFixture(t)
	user := f.user(t)
	shinjuku := f.station(t, 13, "新宿")

	rec := f.update(user, []string{shinjuku.String()}, strings.Repeat("あ", 201))

	assertStatus(t, rec, http.StatusUnprocessableEntity)
	assertContains(t, rec.Body.String(), "200文字以内で入力してください", checkbox(shinjuku, true), `aria-invalid="true"`)
	if got := f.selectedNames(t, user.ID); len(got) != 0 {
		t.Errorf("交換場所の駅 = %v、保存しないことを期待", got)
	}
}

// TestUpdate_WithoutUser は、RequireAuth を通さずに届いたリクエストで誰かの交換場所を保存しないことを検証する。
func TestUpdate_WithoutUser(t *testing.T) {
	t.Parallel()

	assertStatus(t, newCommittedFixture(t).update(nil, nil, ""), http.StatusInternalServerError)
}
