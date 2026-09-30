package trade_withdrawal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/trade_withdrawal"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// postCreate はユーザー user として POST /trades/{id}/withdrawal を処理した応答を返す。
// 取り下げのUseCaseは自分でトランザクションを開くため、コミットした行を読み書きする。
func postCreate(user *model.User, id string) *httptest.ResponseRecorder {
	db := testutil.GetTestDB()
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	handler := trade_withdrawal.NewHandler(
		httperror.NewRenderer(cfg),
		session.NewFlashManager(),
		usecase.NewWithdrawTradeUsecase(db, repository.NewTradeRepository(db), repository.NewTradeEventRepository(db)),
	)

	req := httptest.NewRequest(http.MethodPost, "/trades/"+id+"/withdrawal", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	ctx := context.WithValue(i18n.SetLocale(req.Context(), i18n.LangJa), chi.RouteCtxKey, routeCtx)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}
	rec := httptest.NewRecorder()
	handler.Create(rec, req.WithContext(ctx))

	return rec
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

// tradeStatus は交換 tradeID の段階を返す。
func tradeStatus(t *testing.T, tradeID model.TradeID) model.TradeStatus {
	t.Helper()

	trade, err := repository.NewTradeRepository(testutil.GetTestDB()).FindByID(context.Background(), tradeID)
	if err != nil || trade == nil {
		t.Fatalf("交換の取得に失敗しました: (%+v, %v)", trade, err)
	}

	return trade.Status
}

// TestCreate は、申し込んだ人が返事待ちの交換を取り下げ、完了のメッセージを付けて交換のページへ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	proposer := &model.User{ID: testutil.NewUserBuilder(t, db).Build()}
	tradeID := testutil.NewTradeBuilder(t, db, proposer.ID, testutil.NewUserBuilder(t, db).Build()).Build()

	rec := postCreate(proposer, tradeID.String())

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() {
		t.Fatalf("応答 = %d %q、303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if !flashSet(rec) {
		t.Error("完了のメッセージが書き込まれていない")
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusWithdrawn {
		t.Errorf("交換の段階 = %q、期待値 = %q", status, model.TradeStatusWithdrawn)
	}
}

// TestCreate_Conflict は、返事待ちでなくなった交換は取り下げず、そのことを伝えて交換のページへ戻すことを検証する。
func TestCreate_Conflict(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	proposer := &model.User{ID: testutil.NewUserBuilder(t, db).Build()}
	tradeID := testutil.NewTradeBuilder(t, db, proposer.ID, testutil.NewUserBuilder(t, db).Build()).WithStatus(model.TradeStatusMatched).Build()

	rec := postCreate(proposer, tradeID.String())

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() {
		t.Fatalf("応答 = %d %q、303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if !flashSet(rec) {
		t.Error("取り下げられなかったことを伝えるメッセージが書き込まれていない")
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusMatched {
		t.Errorf("交換の段階 = %q、マッチ成立のままを期待", status)
	}
}

// TestCreate_NotFound は、交換の2人以外・申し込まれた人・無い交換・読めないIDを存在しないページとして扱い、取り下げないことを検証する。
func TestCreate_NotFound(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiver := &model.User{ID: testutil.NewUserBuilder(t, db).Build()}
	tradeID := testutil.NewTradeBuilder(t, db, proposerID, receiver.ID).Build()

	tests := []struct {
		name string
		user *model.User
		id   string
	}{
		{name: "交換の2人以外", user: &model.User{ID: testutil.NewUserBuilder(t, db).Build()}, id: tradeID.String()},
		{name: "申し込まれた人", user: receiver, id: tradeID.String()},
		{name: "無い交換", user: receiver, id: "0199a2b0-0000-7000-8000-000000000000"},
		{name: "読めないID", user: receiver, id: "not-a-uuid"},
	}
	for _, tt := range tests {
		if rec := postCreate(tt.user, tt.id); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusNotFound)
		}
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusPending {
		t.Errorf("交換の段階 = %q、返事待ちのままを期待", status)
	}
}

// TestCreate_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestCreate_NoUser(t *testing.T) {
	t.Parallel()

	if rec := postCreate(nil, "0199a2b0-0000-7000-8000-000000000000"); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
