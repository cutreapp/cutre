package usecase_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newApproveTradeUsecase は、自分でトランザクションを開く ApproveTradeUsecase をテスト用のデータベースで組み立てる。
func newApproveTradeUsecase() *usecase.ApproveTradeUsecase {
	db := testutil.GetTestDB()

	return usecase.NewApproveTradeUsecase(db, repository.NewMessageConsentRepository(db), repository.NewTradeRepository(db), repository.NewTradeEventRepository(db))
}

// TestApproveTradeUsecase_Execute は、同意した申し込まれた人が返事待ちの交換を承認してマッチ成立にし、承認の出来事を記録することを検証する。
func TestApproveTradeUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newApproveTradeUsecase()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	tradeID := testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), receiverID).Build()

	if err := uc.Execute(t.Context(), usecase.ApproveTradeInput{UserID: receiverID, TradeID: tradeID}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
	if err != nil || trade.Status != model.TradeStatusMatched || trade.MatchedAt == nil {
		t.Errorf("承認したあとの交換 = (%+v, %v)、承認した日時のあるマッチ成立を期待", trade, err)
	}
	events, err := repository.NewTradeEventRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(events) != 1 || events[0].Kind != model.TradeEventKindApproved || events[0].ActorUserID != receiverID || events[0].Reason != nil {
		t.Errorf("交換の出来事 = (%+v, %v)、申し込まれた人の、理由の無い承認の1件を期待", events, err)
	}
}

// TestApproveTradeUsecase_Execute_Rejected は、承認できないときに段階を変えず、出来事も記録しないことを検証する。
// 返事待ちでない交換では、同意が無くても同意より先にそのことを返す。
func TestApproveTradeUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newApproveTradeUsecase()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, proposerID).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	withdrawnConsentID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, withdrawnConsentID).WithWithdrawnAt(time.Now()).Build()
	noConsentID := testutil.NewUserBuilder(t, db).Build()
	pending := testutil.NewTradeBuilder(t, db, proposerID, receiverID).Build()
	withdrawn := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusWithdrawn).Build()
	toWithdrawnConsent := testutil.NewTradeBuilder(t, db, proposerID, withdrawnConsentID).Build()
	toNoConsent := testutil.NewTradeBuilder(t, db, proposerID, noConsentID).Build()
	endedToNoConsent := testutil.NewTradeBuilder(t, db, proposerID, noConsentID).WithStatus(model.TradeStatusWithdrawn).Build()

	tests := []struct {
		name     string
		input    usecase.ApproveTradeInput
		wantCode model.AppErrorCode
	}{
		{name: "無い交換", input: usecase.ApproveTradeInput{UserID: receiverID, TradeID: model.TradeID(uuid.New())}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "交換の2人以外", input: usecase.ApproveTradeInput{UserID: testutil.NewUserBuilder(t, db).Build(), TradeID: pending}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "申し込んだ人", input: usecase.ApproveTradeInput{UserID: proposerID, TradeID: pending}, wantCode: model.AppErrCodeForbidden},
		{name: "返事待ちでなくなった交換", input: usecase.ApproveTradeInput{UserID: receiverID, TradeID: withdrawn}, wantCode: model.AppErrCodeConflict},
		{name: "同意をやめた人", input: usecase.ApproveTradeInput{UserID: withdrawnConsentID, TradeID: toWithdrawnConsent}, wantCode: model.AppErrCodeMessageConsentRequired},
		{name: "同意したことが無い人", input: usecase.ApproveTradeInput{UserID: noConsentID, TradeID: toNoConsent}, wantCode: model.AppErrCodeMessageConsentRequired},
		{name: "同意が無い人の終わった交換", input: usecase.ApproveTradeInput{UserID: noConsentID, TradeID: endedToNoConsent}, wantCode: model.AppErrCodeConflict},
	}
	for _, tt := range tests {
		err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != tt.wantCode {
			t.Errorf("%s: エラー = %v、コード %d を期待", tt.name, err, tt.wantCode)
		}
	}

	for _, tradeID := range []model.TradeID{pending, withdrawn, toWithdrawnConsent, toNoConsent} {
		if count := countTradeRows(t, "trade_events", tradeID); count != 0 {
			t.Errorf("交換 %s の出来事の数 = %d、期待値 = 0", tradeID, count)
		}
	}
	for _, tradeID := range []model.TradeID{pending, toWithdrawnConsent, toNoConsent} {
		trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
		if err != nil || trade.Status != model.TradeStatusPending {
			t.Errorf("承認しなかった交換 = (%+v, %v)、返事待ちのままを期待", trade, err)
		}
	}
}
