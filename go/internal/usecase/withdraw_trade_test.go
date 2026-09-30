package usecase_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newWithdrawTradeUsecase は、自分でトランザクションを開く WithdrawTradeUsecase をテスト用のデータベースで組み立てる。
func newWithdrawTradeUsecase() *usecase.WithdrawTradeUsecase {
	db := testutil.GetTestDB()

	return usecase.NewWithdrawTradeUsecase(db, repository.NewTradeRepository(db), repository.NewTradeEventRepository(db))
}

// TestWithdrawTradeUsecase_Execute は、申し込んだ人が返事待ちの交換を取り下げ、取り下げの出来事を記録することを検証する。
func TestWithdrawTradeUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newWithdrawTradeUsecase()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	tradeID := testutil.NewTradeBuilder(t, db, proposerID, testutil.NewUserBuilder(t, db).Build()).Build()

	if err := uc.Execute(t.Context(), usecase.WithdrawTradeInput{UserID: proposerID, TradeID: tradeID}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
	if err != nil || trade.Status != model.TradeStatusWithdrawn || trade.EndedAt == nil {
		t.Errorf("取り下げたあとの交換 = (%+v, %v)、終わった日時のある取り下げを期待", trade, err)
	}
	events, err := repository.NewTradeEventRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(events) != 1 || events[0].Kind != model.TradeEventKindWithdrawn || events[0].ActorUserID != proposerID {
		t.Errorf("交換の出来事 = (%+v, %v)、申し込んだ人の取り下げの1件を期待", events, err)
	}
}

// TestWithdrawTradeUsecase_Execute_Rejected は、取り下げられないときに段階を変えず、出来事も記録しないことを検証する。
func TestWithdrawTradeUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newWithdrawTradeUsecase()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	pending := testutil.NewTradeBuilder(t, db, proposerID, receiverID).Build()
	matched := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()

	tests := []struct {
		name     string
		input    usecase.WithdrawTradeInput
		wantCode model.AppErrorCode
	}{
		{name: "無い交換", input: usecase.WithdrawTradeInput{UserID: proposerID, TradeID: model.TradeID(uuid.New())}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "交換の2人以外", input: usecase.WithdrawTradeInput{UserID: testutil.NewUserBuilder(t, db).Build(), TradeID: pending}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "申し込まれた人", input: usecase.WithdrawTradeInput{UserID: receiverID, TradeID: pending}, wantCode: model.AppErrCodeForbidden},
		{name: "返事待ちでなくなった交換", input: usecase.WithdrawTradeInput{UserID: proposerID, TradeID: matched}, wantCode: model.AppErrCodeConflict},
	}
	for _, tt := range tests {
		err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != tt.wantCode {
			t.Errorf("%s: エラー = %v、コード %d を期待", tt.name, err, tt.wantCode)
		}
	}

	for _, tradeID := range []model.TradeID{pending, matched} {
		if count := countTradeRows(t, "trade_events", tradeID); count != 0 {
			t.Errorf("交換 %s の出来事の数 = %d、期待値 = 0", tradeID, count)
		}
	}
	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), pending)
	if err != nil || trade.Status != model.TradeStatusPending {
		t.Errorf("取り下げなかった交換 = (%+v, %v)、返事待ちのままを期待", trade, err)
	}
}
