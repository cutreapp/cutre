package trade_test

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
	"github.com/cutreapp/cutre/go/internal/handler/trade"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// queryRower はテスト用の行を作る先。テストのトランザクション (*sql.Tx) とコミットする接続 (*sql.DB) のどちらも渡せる。
type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// newHandler は Handler を組み立てる。画面に出すものは tx の中で読み、tx がnilならコミットした行を読む。
// 申し込みのUseCaseは自分でトランザクションを開くため、常にコミットした行を読み書きする。
func newHandler(tx *sql.Tx) *trade.Handler {
	db := testutil.GetTestDB()
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	eventCategoryRepo := repository.NewEventCategoryRepository(db)
	goodsRepo := repository.NewGoodsRepository(db)
	itemRepo := repository.NewItemRepository(db)
	messageConsentRepo := repository.NewMessageConsentRepository(db)
	userRepo := repository.NewUserRepository(db)
	stationRepo := repository.NewStationRepository(db)
	tradeRepo := repository.NewTradeRepository(db)
	tradeEventRepo := repository.NewTradeEventRepository(db)
	tradeItemRepo := repository.NewTradeItemRepository(db)
	tradeMessageRepo := repository.NewTradeMessageRepository(db)
	userStationRepo := repository.NewUserStationRepository(db)
	// 画面を描くUseCaseは tx の中で読む。申し込みのUseCaseは自分でトランザクションを開くため、ここで作ったリポジトリを使う。
	readEventCategoryRepo, readGoodsRepo, readItemRepo, readMessageConsentRepo, readUserRepo := eventCategoryRepo, goodsRepo, itemRepo, messageConsentRepo, userRepo
	readStationRepo, readTradeRepo, readTradeEventRepo, readTradeItemRepo, readUserStationRepo := stationRepo, tradeRepo, tradeEventRepo, tradeItemRepo, userStationRepo
	readTradeMessageRepo := tradeMessageRepo
	if tx != nil {
		readEventCategoryRepo, readGoodsRepo, readItemRepo = eventCategoryRepo.WithTx(tx), goodsRepo.WithTx(tx), itemRepo.WithTx(tx)
		readMessageConsentRepo, readUserRepo = messageConsentRepo.WithTx(tx), userRepo.WithTx(tx)
		readStationRepo, readTradeRepo, readTradeEventRepo = stationRepo.WithTx(tx), tradeRepo.WithTx(tx), tradeEventRepo.WithTx(tx)
		readTradeItemRepo, readUserStationRepo = tradeItemRepo.WithTx(tx), userStationRepo.WithTx(tx)
		readTradeMessageRepo = tradeMessageRepo.WithTx(tx)
	}

	return trade.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		usecase.NewGetTradesUsecase(readItemRepo, readTradeRepo, readTradeItemRepo, readUserRepo),
		usecase.NewGetMatchesUsecase(readEventCategoryRepo, readGoodsRepo, readItemRepo, readStationRepo, readUserRepo, readUserStationRepo),
		usecase.NewGetTradeUsecase(readEventCategoryRepo, readGoodsRepo, readItemRepo, readMessageConsentRepo, readTradeRepo, readTradeEventRepo, readTradeItemRepo, readTradeMessageRepo, readUserRepo),
		usecase.NewGetTradeProposalUsecase(readEventCategoryRepo, readGoodsRepo, readItemRepo, readMessageConsentRepo, readUserRepo),
		usecase.NewCreateTradeUsecase(
			db,
			validator.NewTradeCreateValidator(itemRepo),
			messageConsentRepo,
			tradeRepo,
			tradeEventRepo,
			tradeItemRepo,
			tradeMessageRepo,
			userRepo,
		),
	)
}

// withContext は、ログイン中のユーザー・日本語のロケール・ルーターがパスから取り出したアットネームを載せたリクエストを返す。
// user がnilのときは RequireAuth を通していないリクエストとして、ユーザーを載せない。
func withContext(req *http.Request, user *model.User, atname string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("atname", atname)
	ctx := context.WithValue(i18n.SetLocale(req.Context(), i18n.LangJa), chi.RouteCtxKey, routeCtx)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}

	return req.WithContext(ctx)
}

// fixture は申し込みのテストに使う、申し込む人と、申し込まれる人と、2人の間で交換できるアイテム。
type fixture struct {
	proposer *model.User
	receiver *model.User
	// receiveItemID は申し込まれた人の譲れるアイテムで、申し込む人がほしいもの。
	receiveItemID model.ItemID
	// giveItemID は申し込む人の譲れるアイテムで、申し込まれた人がほしいもの。
	giveItemID model.ItemID
}

// newFixture は、申し込む人と申し込まれる人のそれぞれに、相手がほしいグッズを譲れるリストに入れて、db に作る。
// consent がtrueなら申し込む人を同意済みにする。
func newFixture(t *testing.T, db queryRower, consent bool) fixture {
	t.Helper()

	categoryID := testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).WithName("B賞").Build()
	wantedGoods := testutil.NewGoodsBuilder(t, db, categoryID).WithName("りすの子").Build()
	offeredGoods := testutil.NewGoodsBuilder(t, db, categoryID).WithName("くまの子").Build()
	proposer := newUser(t, db)
	receiver := newUser(t, db)
	if consent {
		testutil.NewMessageConsentBuilder(t, db, proposer.ID).Build()
	}
	testutil.NewItemBuilder(t, db, proposer.ID, wantedGoods).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, db, receiver.ID, offeredGoods).WithKind(model.ItemKindWant).Build()

	return fixture{
		proposer:      proposer,
		receiver:      receiver,
		receiveItemID: testutil.NewItemBuilder(t, db, receiver.ID, wantedGoods).WithNote("未開封です").Build(),
		giveItemID:    testutil.NewItemBuilder(t, db, proposer.ID, offeredGoods).Build(),
	}
}

// newUser は db にユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func newUser(t *testing.T, db queryRower) *model.User {
	t.Helper()

	atname := testutil.UniqueAtname()
	return &model.User{ID: testutil.NewUserBuilder(t, db).WithAtname(atname).Build(), Atname: atname, Locale: model.LocaleJa}
}

// postCreate はユーザー user として POST /trades を処理した応答を返す。
func postCreate(handler *trade.Handler, user *model.User, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/trades", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.Create(rec, withContext(req, user, ""))

	return rec
}

// assertContains はボディに want のすべてが含まれることを検証する。
func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}
