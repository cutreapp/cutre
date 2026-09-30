package event_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/event"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// fixture はテスト用のトランザクションの中で動く Handler と、そのトランザクション。
// イベントのUseCaseは自分でトランザクションを開かないため、テストのトランザクションで包める。
type fixture struct {
	handler *event.Handler
	tx      *sql.Tx
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	eventRepo := repository.NewEventRepository(db).WithTx(tx)
	categoryRepo := repository.NewEventCategoryRepository(db).WithTx(tx)
	goodsRepo := repository.NewGoodsRepository(db).WithTx(tx)
	itemRepo := repository.NewItemRepository(db).WithTx(tx)
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	return &fixture{
		handler: event.NewHandler(
			cfg,
			httperror.NewRenderer(cfg),
			usecase.NewGetEventsUsecase(eventRepo, goodsRepo, itemRepo),
			usecase.NewGetEventUsecase(eventRepo, categoryRepo, goodsRepo, itemRepo),
		),
		tx: tx,
	}
}

// user はユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func (f *fixture) user(t *testing.T) *model.User {
	t.Helper()

	id := testutil.NewUserBuilder(t, f.tx).Build()
	user, err := repository.NewUserRepository(testutil.GetTestDB()).WithTx(f.tx).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// request は user がログインしたGETのリクエストを作る。URLの {event_id} に eventID を載せる。
func request(target string, user *model.User, eventID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("event_id", eventID)
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, user)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
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

// TestIndex は、公開中のイベントをカテゴリーの一覧へのリンクにし、開催期間・グッズの種類の数・自分が入れた数量を添え、
// アーカイブしたイベントを出さないことを検証する。
func TestIndex(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.user(t)
	eventID := testutil.NewEventBuilder(t, f.tx).WithName("ふわりす もちもちくじ").Build()
	testutil.NewEventBuilder(t, f.tx).WithName("終わったくじ").WithArchived("終わったため").Build()
	goodsID := testutil.NewGoodsBuilder(t, f.tx, testutil.NewEventCategoryBuilder(t, f.tx, eventID).Build()).Build()
	testutil.NewItemBuilder(t, f.tx, user.ID, goodsID).WithQuantity(3).Build()

	rec := httptest.NewRecorder()
	f.handler.Index(rec, request("/events", user, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, `href="/events/`+eventID.String()+`"`, "ふわりす もちもちくじ", "全1種", "譲れる 3点", `href="/home"`)
	if strings.Contains(body, "終わったくじ") {
		t.Error("アーカイブしたイベントを一覧に出した")
	}
}

// TestShow は、公開中のカテゴリーを、グッズの一覧へのリンクにして並べることを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.user(t)
	eventID := testutil.NewEventBuilder(t, f.tx).WithName("ふわりす もちもちくじ").Build()
	categoryID := testutil.NewEventCategoryBuilder(t, f.tx, eventID).WithName("B賞 ラバーマスコット").Build()
	testutil.NewEventCategoryBuilder(t, f.tx, eventID).WithName("外れたカテゴリー").WithArchived("景品から外れたため").Build()
	testutil.NewGoodsBuilder(t, f.tx, categoryID).Build()

	rec := httptest.NewRecorder()
	f.handler.Show(rec, request("/events/"+eventID.String(), user, eventID.String()))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "ふわりす もちもちくじ", `href="/events/`+eventID.String()+`/categories/`+categoryID.String()+`"`, "B賞 ラバーマスコット", "1種", `href="/events"`)
	if strings.Contains(body, "外れたカテゴリー") {
		t.Error("アーカイブしたカテゴリーを一覧に出した")
	}
}

// TestShow_NotFound は、UUIDとして読めないIDと、公開していないイベントに404を返すことを検証する。
func TestShow_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.user(t)

	for name, id := range map[string]string{
		"UUIDとして読めない": "not-a-uuid",
		"アーカイブしたイベント": testutil.NewEventBuilder(t, f.tx).WithArchived("終わったため").Build().String(),
	} {
		rec := httptest.NewRecorder()
		f.handler.Show(rec, request("/events/"+id, user, id))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestIndex_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かの一覧として描画しないことを検証する。
func TestIndex_WithoutUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.Index(rec, request("/events", nil, ""))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
