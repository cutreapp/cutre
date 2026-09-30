package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newCancelTradeUsecase は、自分でトランザクションを開く CancelTradeUsecase をテスト用のデータベースで組み立てる。
func newCancelTradeUsecase() *usecase.CancelTradeUsecase {
	db := testutil.GetTestDB()

	return usecase.NewCancelTradeUsecase(
		db,
		validator.NewTradeCancellationValidator(),
		repository.NewMessageConsentRepository(db),
		repository.NewTradeRepository(db),
		repository.NewTradeEventRepository(db),
		repository.NewTradeMessageRepository(db),
	)
}

// TestCancelTradeUsecase_Execute は、交換の2人のどちらかが、どちらも「交換できた」を押していないマッチ成立の交換をやめ、
// 理由とともに出来事を記録して、前後の空白を除いたひとことをメッセージとして残すことを検証する。
func TestCancelTradeUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newCancelTradeUsecase()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	tradeID := testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), receiverID).WithStatus(model.TradeStatusMatched).Build()

	err := uc.Execute(jaContext(), usecase.CancelTradeInput{UserID: receiverID, TradeID: tradeID, Reason: "decided_elsewhere", Note: " 先に別の方と決まってしまいました。 "})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
	if err != nil || trade.Status != model.TradeStatusCancelled || trade.EndedAt == nil {
		t.Errorf("やめたあとの交換 = (%+v, %v)、終わった日時のある「やめた」を期待", trade, err)
	}
	events, err := repository.NewTradeEventRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(events) != 1 || events[0].Kind != model.TradeEventKindCancelled || events[0].ActorUserID != receiverID ||
		events[0].Reason == nil || *events[0].Reason != "decided_elsewhere" {
		t.Errorf("交換の出来事 = (%+v, %v)、申し込まれた人の「ほかの人と交換が決まった」でやめた1件を期待", events, err)
	}
	messages, err := repository.NewTradeMessageRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(messages) != 1 || messages[0].SenderUserID != receiverID || messages[0].Body != "先に別の方と決まってしまいました。" {
		t.Errorf("交換のメッセージ = (%+v, %v)、申し込まれた人の前後の空白を除いたひとことの1通を期待", messages, err)
	}
}

// TestCancelTradeUsecase_Execute_Rejected は、やめられないときに段階を変えず、出来事もメッセージも記録しないことを検証する。
// マッチ成立でない交換と、どちらかが「交換できた」を押した交換では、入力の誤りや同意より先にそのことを返す。
func TestCancelTradeUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newCancelTradeUsecase()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	noConsentID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, noConsentID).WithWithdrawnAt(time.Now()).Build()
	matched := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()
	halfCompleted := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).WithProposerCompletedAt(time.Now()).Build()
	failed := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusFailed).Build()
	toNoConsent := testutil.NewTradeBuilder(t, db, proposerID, noConsentID).WithStatus(model.TradeStatusMatched).Build()

	tests := []struct {
		name     string
		input    usecase.CancelTradeInput
		wantCode model.AppErrorCode
	}{
		{name: "無い交換", input: usecase.CancelTradeInput{UserID: receiverID, TradeID: model.TradeID(uuid.New()), Reason: "other", Note: "ごめんなさい"}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "交換の2人以外", input: usecase.CancelTradeInput{UserID: testutil.NewUserBuilder(t, db).Build(), TradeID: matched, Reason: "other", Note: "ごめんなさい"}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "相手が「交換できた」を押した交換", input: usecase.CancelTradeInput{UserID: receiverID, TradeID: halfCompleted}, wantCode: model.AppErrCodeConflict},
		{name: "終わった交換", input: usecase.CancelTradeInput{UserID: receiverID, TradeID: failed}, wantCode: model.AppErrCodeConflict},
		{name: "同意の無い人", input: usecase.CancelTradeInput{UserID: noConsentID, TradeID: toNoConsent, Reason: "other", Note: "ごめんなさい"}, wantCode: model.AppErrCodeMessageConsentRequired},
	}
	for _, tt := range tests {
		err := uc.Execute(jaContext(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != tt.wantCode {
			t.Errorf("%s: エラー = %v、コード %d を期待", tt.name, err, tt.wantCode)
		}
	}

	err := uc.Execute(jaContext(), usecase.CancelTradeInput{UserID: receiverID, TradeID: matched, Reason: "other"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("note") {
		t.Errorf("ひとことが空: エラー = %v、ひとことの欄のエラーを期待", err)
	}

	for _, tradeID := range []model.TradeID{matched, halfCompleted, failed, toNoConsent} {
		if count := countTradeRows(t, "trade_events", tradeID) + countTradeRows(t, "trade_messages", tradeID); count != 0 {
			t.Errorf("交換 %s の出来事とメッセージの数 = %d、期待値 = 0", tradeID, count)
		}
	}
	for _, tradeID := range []model.TradeID{matched, halfCompleted, toNoConsent} {
		trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
		if err != nil || trade.Status != model.TradeStatusMatched {
			t.Errorf("やめなかった交換 = (%+v, %v)、マッチ成立のままを期待", trade, err)
		}
	}
}

// TestCancelTradeUsecase_Execute_CompletedMeanwhile は、読んだあとに相手が「交換できた」を押していたとき、
// 押した更新のコミットを待ってから更新の条件で確かめ直し、交換をやめずに AppErrCodeConflict を返すことを検証する。
func TestCancelTradeUsecase_Execute_CompletedMeanwhile(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	tradeID := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()
	ctx, cancel := context.WithTimeout(jaContext(), 5*time.Second)
	defer cancel()

	// 申し込んだ人が「交換できた」を押した更新をコミットせずに保ち、申し込まれた人のやめる操作を待たせる。
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if updated, err := repository.NewTradeRepository(db).WithTx(tx).Complete(ctx, tradeID, proposerID); err != nil || updated == nil {
		t.Fatalf("先の Complete() = (%+v, %v)、押した交換を期待", updated, err)
	}

	second := make(chan error, 1)
	go func() {
		second <- newCancelTradeUsecase().Execute(ctx, usecase.CancelTradeInput{UserID: receiverID, TradeID: tradeID, Reason: "other", Note: "ごめんなさい"})
	}()
	select {
	case err := <-second:
		t.Fatalf("先の更新のコミット前にやめる操作が完了した: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := <-second; model.AsAppError(err) == nil || model.AsAppError(err).Code != model.AppErrCodeConflict {
		t.Fatalf("あとの Execute()のエラー = %v、AppErrCodeConflict を期待", err)
	}
	trade, err := repository.NewTradeRepository(db).FindByID(ctx, tradeID)
	if err != nil || trade.Status != model.TradeStatusMatched || trade.ProposerCompletedAt == nil {
		t.Errorf("やめる操作のあとの交換 = (%+v, %v)、申し込んだ人だけが押したマッチ成立を期待", trade, err)
	}
	if count := countTradeRows(t, "trade_messages", tradeID); count != 0 {
		t.Errorf("交換のメッセージの数 = %d、期待値 = 0", count)
	}
}
