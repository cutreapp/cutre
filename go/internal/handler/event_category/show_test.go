package event_category_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/event_category"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// request は user がログインしたGETのリクエストを作る。URLの {event_id} と {category_id} に値を載せる。
func request(user *model.User, eventID, categoryID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/events/"+eventID+"/categories/"+categoryID, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("event_id", eventID)
	routeCtx.URLParams.Add("category_id", categoryID)
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, user)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// TestShow は、公開中のグッズごとに、自分のリストに入れた数量と、入れていないリストへの追加の入口を出すことを検証する。
// 入れているリストのボタンは押した状態で示し、リンクにしない。
func TestShow(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo := repository.NewEventRepository(db).WithTx(tx)
	categoryRepo := repository.NewEventCategoryRepository(db).WithTx(tx)
	goodsRepo := repository.NewGoodsRepository(db).WithTx(tx)
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	handler := event_category.NewHandler(cfg, httperror.NewRenderer(cfg), usecase.NewGetEventCategoryUsecase(eventRepo, categoryRepo, goodsRepo, repository.NewItemRepository(db).WithTx(tx)))

	userID := testutil.NewUserBuilder(t, tx).Build()
	user, err := repository.NewUserRepository(db).WithTx(tx).FindByID(context.Background(), userID)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}
	eventID := testutil.NewEventBuilder(t, tx).WithName("ふわりす もちもちくじ").Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithName("B賞 ラバーマスコット").Build()
	listedID := testutil.NewGoodsBuilder(t, tx, categoryID).WithName("くまの子").WithPosition(1).Build()
	unlistedID := testutil.NewGoodsBuilder(t, tx, categoryID).WithName("ねこの子").WithPosition(2).Build()
	testutil.NewGoodsBuilder(t, tx, categoryID).WithName("外れた子").WithArchived("景品から外れたため").Build()
	listedItemID := testutil.NewItemBuilder(t, tx, userID, listedID).WithQuantity(2).Build()

	rec := httptest.NewRecorder()
	handler.Show(rec, request(user, eventID.String(), categoryID.String()))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"B賞 ラバーマスコット",
		`href="/events/` + eventID.String() + `"`,
		"全2種",
		"譲れるリストに2点",
		"リストになし",
		`href="/items/` + listedItemID.String() + `/edit" class="btn flex-1 gap-1" data-size="sm" data-listed aria-label="譲れるリストのくまの子を編集"`,
		`href="/items/new?goods_id=` + listedID.String() + `&amp;kind=want"`,
		`href="/items/new?goods_id=` + unlistedID.String() + `&amp;kind=give"`,
		`aria-label="ねこの子を譲れるリストに追加"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	for _, absent := range []string{`href="/items/new?goods_id=` + listedID.String() + `&amp;kind=give"`, "外れた子", `<span class="btn flex-1 gap-1" data-size="sm" aria-current="true"`} {
		if strings.Contains(body, absent) {
			t.Errorf("レスポンスボディに %q が含まれている", absent)
		}
	}

	// カテゴリーをほかのイベントのURLで開いたときは、存在しないページとして扱う。
	rec = httptest.NewRecorder()
	handler.Show(rec, request(user, testutil.NewEventBuilder(t, tx).Build().String(), categoryID.String()))
	if rec.Code != http.StatusNotFound {
		t.Errorf("ほかのイベントのURLのステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}
