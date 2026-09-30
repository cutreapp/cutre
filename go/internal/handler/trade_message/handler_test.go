package trade_message_test

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
	"github.com/cutreapp/cutre/go/internal/handler/trade_message"
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

// newHandler はテストのトランザクション tx で読み書きする Handler を組み立てる。レート制限はコミットした行で数える。
func newHandler(tx *sql.Tx) *trade_message.Handler {
	db := testutil.GetTestDB()
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	messageConsentRepo := repository.NewMessageConsentRepository(db).WithTx(tx)
	tradeRepo := repository.NewTradeRepository(db).WithTx(tx)
	tradeMessageRepo := repository.NewTradeMessageRepository(db).WithTx(tx)
	tradeMessageReadRepo := repository.NewTradeMessageReadRepository(db).WithTx(tx)

	return trade_message.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		usecase.NewGetTradeMessagesUsecase(
			repository.NewItemRepository(db).WithTx(tx),
			messageConsentRepo,
			tradeRepo,
			repository.NewTradeEventRepository(db).WithTx(tx),
			repository.NewTradeItemRepository(db).WithTx(tx),
			tradeMessageRepo,
			tradeMessageReadRepo,
			repository.NewUserRepository(db).WithTx(tx),
		),
		usecase.NewMarkTradeMessagesReadUsecase(tradeRepo, tradeMessageReadRepo),
		usecase.NewCreateTradeMessageUsecase(validator.NewTradeMessageCreateValidator(), messageConsentRepo, tradeRepo, tradeMessageRepo),
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

// getIndex はユーザー user として GET /trades/{id}/messages を処理した応答を返す。
func getIndex(tx *sql.Tx, user *model.User, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/trades/"+id+"/messages", nil)
	rec := httptest.NewRecorder()
	newHandler(tx).Index(rec, withContext(req, user, id))

	return rec
}

// postCreate はユーザー user として本文 body で POST /trades/{id}/messages を処理した応答を返す。
func postCreate(tx *sql.Tx, user *model.User, id string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/trades/"+id+"/messages", strings.NewReader(url.Values{"body": {body}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	newHandler(tx).Create(rec, withContext(req, user, id))

	return rec
}

// fixture はメッセージのテストに使う、交換の2人と、その間の交換。
type fixture struct {
	proposer *model.User
	receiver *model.User
	tradeID  model.TradeID
}

// newFixture は、同意済みの2人の間に、状態 status の交換を、2人の譲れるアイテムを品にして、申し込みの出来事とひとことのメッセージとともに tx に作る。
func newFixture(t *testing.T, tx *sql.Tx, status model.TradeStatus) fixture {
	t.Helper()

	db := testutil.GetTestDB()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()
	proposer := newUser(t, tx)
	receiver := newUser(t, tx)
	testutil.NewMessageConsentBuilder(t, tx, proposer.ID).Build()
	testutil.NewMessageConsentBuilder(t, tx, receiver.ID).Build()
	receiveItemID := testutil.NewItemBuilder(t, tx, receiver.ID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).Build()
	giveItemID := testutil.NewItemBuilder(t, tx, proposer.ID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposer.ID, receiver.ID).WithStatus(status).Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), tradeID, []model.ItemID{receiveItemID, giveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	if _, err := repository.NewTradeEventRepository(db).WithTx(tx).Create(t.Context(), tradeID, proposer.ID, model.TradeEventKindProposed, nil); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	testutil.NewTradeMessageBuilder(t, tx, tradeID, proposer.ID, "はじめまして。\n<b>土日</b>なら動けます。").WithCreatedAt(time.Now().Add(-time.Hour)).Build()

	return fixture{proposer: proposer, receiver: receiver, tradeID: tradeID}
}

// newUser は日本語で使うユーザーを db に作る。
func newUser(t *testing.T, db *sql.Tx) *model.User {
	t.Helper()

	atname := testutil.UniqueAtname()
	return &model.User{ID: testutil.NewUserBuilder(t, db).WithAtname(atname).Build(), Atname: atname, Locale: model.LocaleJa}
}

// messages は交換 tradeID のメッセージを、送った順に返す。
func messages(t *testing.T, tx *sql.Tx, tradeID model.TradeID) []*model.TradeMessage {
	t.Helper()

	found, err := repository.NewTradeMessageRepository(testutil.GetTestDB()).WithTx(tx).ListByTradeID(t.Context(), tradeID)
	if err != nil {
		t.Fatalf("ListByTradeID()のエラー = %v", err)
	}

	return found
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

// assertContains はボディに want のすべてが含まれることを検証する。
func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}
