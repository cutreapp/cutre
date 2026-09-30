package trade_cancellation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/trade_cancellation"
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

// newHandler は Handler を組み立てる。交換をやめるのUseCaseは自分でトランザクションを開くため、画面も含めてコミットした行を読み書きする。
func newHandler() *trade_cancellation.Handler {
	db := testutil.GetTestDB()
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	messageConsentRepo := repository.NewMessageConsentRepository(db)
	tradeRepo := repository.NewTradeRepository(db)
	tradeEventRepo := repository.NewTradeEventRepository(db)
	tradeMessageRepo := repository.NewTradeMessageRepository(db)

	return trade_cancellation.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		session.NewFlashManager(),
		usecase.NewGetTradeUsecase(
			repository.NewEventCategoryRepository(db),
			repository.NewGoodsRepository(db),
			repository.NewItemRepository(db),
			messageConsentRepo,
			tradeRepo,
			tradeEventRepo,
			repository.NewTradeItemRepository(db),
			tradeMessageRepo,
			repository.NewUserRepository(db),
		),
		usecase.NewCancelTradeUsecase(db, validator.NewTradeCancellationValidator(), messageConsentRepo, tradeRepo, tradeEventRepo, tradeMessageRepo),
	)
}

// withContext は、ログイン中のユーザー・日本語のロケール・ルーターがパスから取り出した交換のIDを載せたリクエストを返す。
// user がnilのときは RequireAuth を通していないリクエストとして、ユーザーを載せない。
func withContext(req *http.Request, user *model.User, id string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	ctx := context.WithValue(i18n.SetLocale(req.Context(), i18n.LangJa), chi.RouteCtxKey, routeCtx)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}

	return req.WithContext(ctx)
}

// newUser はアットネームを持つユーザーを作る。consent がtrueならメッセージの取り扱いに同意させる。
func newUser(t *testing.T, consent bool) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	atname := testutil.UniqueAtname()
	user := &model.User{ID: testutil.NewUserBuilder(t, db).WithAtname(atname).Build(), Atname: atname, Locale: model.LocaleJa}
	if consent {
		testutil.NewMessageConsentBuilder(t, db, user.ID).Build()
	}

	return user
}

// newTrade は、申し込んだ人 proposer と申し込まれた人 receiver のマッチ成立の交換を作る。
// partnerCompleted がtrueなら、申し込んだ人が「交換できた」を押した交換にする。
func newTrade(t *testing.T, proposer, receiver *model.User, partnerCompleted bool) model.TradeID {
	t.Helper()

	builder := testutil.NewTradeBuilder(t, testutil.GetTestDB(), proposer.ID, receiver.ID).WithStatus(model.TradeStatusMatched)
	if partnerCompleted {
		builder = builder.WithProposerCompletedAt(time.Now())
	}

	return builder.Build()
}

// tradeStatus は交換 tradeID の段階を返す。
func tradeStatus(t *testing.T, tradeID model.TradeID) model.TradeStatus {
	t.Helper()

	trade, err := repository.NewTradeRepository(testutil.GetTestDB()).FindByID(context.Background(), tradeID)
	if err != nil || trade == nil {
		t.Fatalf("交換の取得に失敗しました: (%+v, %v)", trade, err)
	}

	return trade.Status
}

// flashSet は応答がフラッシュメッセージのCookieを書き込んだかを返す。
func flashSet(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.FlashCookieName && c.Value != "" {
			return true
		}
	}
	return false
}

// assertContains は body が wants をすべて含むことを確かめる。
func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスに %q が含まれていない", want)
		}
	}
}
