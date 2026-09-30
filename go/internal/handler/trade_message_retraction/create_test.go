package trade_message_retraction_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/trade_message_retraction"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// postCreate はユーザー user として POST /trades/{id}/messages/{message_id}/retraction をテストのトランザクション tx で処理した応答を返す。
func postCreate(tx *sql.Tx, user *model.User, tradeID, messageID string) *httptest.ResponseRecorder {
	db := testutil.GetTestDB()
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	handler := trade_message_retraction.NewHandler(
		httperror.NewRenderer(cfg),
		session.NewFlashManager(),
		usecase.NewRetractTradeMessageUsecase(repository.NewTradeRepository(db).WithTx(tx), repository.NewTradeMessageRepository(db).WithTx(tx)),
	)
	req := httptest.NewRequest(http.MethodPost, "/trades/"+tradeID+"/messages/"+messageID+"/retraction", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", tradeID)
	routeCtx.URLParams.Add("message_id", messageID)
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

// retracted はメッセージ messageID が取り消されているかを返す。
func retracted(t *testing.T, tx *sql.Tx, messageID model.TradeMessageID) bool {
	t.Helper()

	message, err := repository.NewTradeMessageRepository(testutil.GetTestDB()).WithTx(tx).FindByID(t.Context(), messageID)
	if err != nil || message == nil {
		t.Fatalf("メッセージの取得に失敗しました: (%+v, %v)", message, err)
	}

	return message.RetractedAt != nil
}

// TestCreate は、送った人がメッセージを取り消し、完了のメッセージを付けてメッセージのページの末尾へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	sender := &model.User{ID: testutil.NewUserBuilder(t, tx).Build()}
	tradeID := testutil.NewTradeBuilder(t, tx, sender.ID, testutil.NewUserBuilder(t, tx).Build()).Build()
	messageID := testutil.NewTradeMessageBuilder(t, tx, tradeID, sender.ID, "取り消す本文").Build()

	rec := postCreate(tx, sender, tradeID.String(), messageID.String())

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if want := "/trades/" + tradeID.String() + "/messages#trade-messages-latest"; rec.Header().Get("Location") != want {
		t.Errorf("Location = %q、期待値 = %q", rec.Header().Get("Location"), want)
	}
	if !flashSet(rec) {
		t.Error("取り消したことを伝えるフラッシュメッセージを付けていない")
	}
	if !retracted(t, tx, messageID) {
		t.Error("メッセージを取り消していない")
	}
}

// TestCreate_NotFound は、相手のメッセージ・交換の2人以外・無いメッセージ・読めないIDを存在しないページとして扱い、取り消さないことを検証する。
func TestCreate_NotFound(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	sender := &model.User{ID: testutil.NewUserBuilder(t, tx).Build()}
	partner := &model.User{ID: testutil.NewUserBuilder(t, tx).Build()}
	tradeID := testutil.NewTradeBuilder(t, tx, sender.ID, partner.ID).Build()
	messageID := testutil.NewTradeMessageBuilder(t, tx, tradeID, sender.ID, "取り消さない本文").Build()

	tests := []struct {
		name      string
		user      *model.User
		tradeID   string
		messageID string
	}{
		{name: "相手のメッセージ", user: partner, tradeID: tradeID.String(), messageID: messageID.String()},
		{name: "交換の2人以外", user: &model.User{ID: testutil.NewUserBuilder(t, tx).Build()}, tradeID: tradeID.String(), messageID: messageID.String()},
		{name: "無いメッセージ", user: sender, tradeID: tradeID.String(), messageID: "0199a2b0-0000-7000-8000-000000000000"},
		{name: "読めない交換のID", user: sender, tradeID: "not-a-uuid", messageID: messageID.String()},
		{name: "読めないメッセージのID", user: sender, tradeID: tradeID.String(), messageID: "not-a-uuid"},
	}
	for _, tt := range tests {
		if rec := postCreate(tx, tt.user, tt.tradeID, tt.messageID); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusNotFound)
		}
	}
	if retracted(t, tx, messageID) {
		t.Error("取り消せないメッセージを取り消した")
	}
}

// TestCreate_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestCreate_NoUser(t *testing.T) {
	t.Parallel()

	if rec := postCreate(nil, nil, "0199a2b0-0000-7000-8000-000000000000", "0199a2b0-0000-7000-8000-000000000001"); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
