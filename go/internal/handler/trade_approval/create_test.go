package trade_approval_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/trade_approval"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// postCreate はユーザー user として POST /trades/{id}/approval を処理した応答を返す。
// 承認のUseCaseは自分でトランザクションを開くため、コミットした行を読み書きする。
func postCreate(user *model.User, id string) *httptest.ResponseRecorder {
	db := testutil.GetTestDB()
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	handler := trade_approval.NewHandler(
		httperror.NewRenderer(cfg),
		session.NewFlashManager(),
		usecase.NewApproveTradeUsecase(db, repository.NewMessageConsentRepository(db), repository.NewTradeRepository(db), repository.NewTradeEventRepository(db)),
	)

	req := httptest.NewRequest(http.MethodPost, "/trades/"+id+"/approval", nil)
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

// newConsentedUser は、メッセージの取り扱いに同意したユーザーを作る。
func newConsentedUser(t *testing.T) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	user := &model.User{ID: testutil.NewUserBuilder(t, db).Build()}
	testutil.NewMessageConsentBuilder(t, db, user.ID).Build()

	return user
}

// TestCreate は、同意した申し込まれた人が返事待ちの交換を承認し、完了のメッセージを付けて交換のページへ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	receiver := newConsentedUser(t)
	tradeID := testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), receiver.ID).Build()

	rec := postCreate(receiver, tradeID.String())

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() {
		t.Fatalf("応答 = %d %q、303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if !flashSet(rec) {
		t.Error("完了のメッセージが書き込まれていない")
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusMatched {
		t.Errorf("交換の段階 = %q、期待値 = %q", status, model.TradeStatusMatched)
	}
}

// TestCreate_Conflict は、返事待ちでなくなった交換は承認せず、そのことを伝えて交換のページへ戻すことを検証する。
func TestCreate_Conflict(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	receiver := newConsentedUser(t)
	tradeID := testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), receiver.ID).WithStatus(model.TradeStatusWithdrawn).Build()

	rec := postCreate(receiver, tradeID.String())

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() {
		t.Fatalf("応答 = %d %q、303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if !flashSet(rec) {
		t.Error("承認できなかったことを伝えるメッセージが書き込まれていない")
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusWithdrawn {
		t.Errorf("交換の段階 = %q、取り下げのままを期待", status)
	}
}

// TestCreate_MessageConsentRequired は、同意の無い申し込まれた人は承認せず、そのことを伝えてメッセージの利用の画面へ送ることを検証する。
func TestCreate_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	receiver := &model.User{ID: testutil.NewUserBuilder(t, db).Build()}
	tradeID := testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), receiver.ID).Build()

	rec := postCreate(receiver, tradeID.String())

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" {
		t.Fatalf("応答 = %d %q、303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if !flashSet(rec) {
		t.Error("同意が必要なことを伝えるメッセージが書き込まれていない")
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusPending {
		t.Errorf("交換の段階 = %q、返事待ちのままを期待", status)
	}
}

// TestCreate_NotFound は、交換の2人以外・申し込んだ人・無い交換・読めないIDを存在しないページとして扱い、承認しないことを検証する。
func TestCreate_NotFound(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	proposer := newConsentedUser(t)
	receiver := newConsentedUser(t)
	tradeID := testutil.NewTradeBuilder(t, db, proposer.ID, receiver.ID).Build()

	tests := []struct {
		name string
		user *model.User
		id   string
	}{
		{name: "交換の2人以外", user: newConsentedUser(t), id: tradeID.String()},
		{name: "申し込んだ人", user: proposer, id: tradeID.String()},
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
