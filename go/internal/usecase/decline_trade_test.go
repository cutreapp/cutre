package usecase_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newDeclineTradeUsecase は、自分でトランザクションを開く DeclineTradeUsecase をテスト用のデータベースで組み立てる。
func newDeclineTradeUsecase() *usecase.DeclineTradeUsecase {
	db := testutil.GetTestDB()

	return usecase.NewDeclineTradeUsecase(
		db,
		validator.NewTradeDeclineValidator(),
		repository.NewMessageConsentRepository(db),
		repository.NewTradeRepository(db),
		repository.NewTradeEventRepository(db),
		repository.NewTradeMessageRepository(db),
	)
}

// TestDeclineTradeUsecase_Execute は、申し込まれた人が返事待ちの交換をお断りし、理由とともにお断りの出来事を記録して、
// 前後の空白を除いたひとことをメッセージとして残すことを検証する。
func TestDeclineTradeUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newDeclineTradeUsecase()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	tradeID := testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), receiverID).Build()

	err := uc.Execute(t.Context(), usecase.DeclineTradeInput{UserID: receiverID, TradeID: tradeID, Reason: "already_decided", Note: " 先に別の方と決まってしまいました。 "})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
	if err != nil || trade.Status != model.TradeStatusDeclined || trade.EndedAt == nil {
		t.Errorf("お断りしたあとの交換 = (%+v, %v)、終わった日時のあるお断りを期待", trade, err)
	}
	events, err := repository.NewTradeEventRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(events) != 1 || events[0].Kind != model.TradeEventKindDeclined || events[0].ActorUserID != receiverID ||
		events[0].Reason == nil || *events[0].Reason != "already_decided" {
		t.Errorf("交換の出来事 = (%+v, %v)、申し込まれた人の「もう交換が決まった」のお断りの1件を期待", events, err)
	}
	messages, err := repository.NewTradeMessageRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(messages) != 1 || messages[0].SenderUserID != receiverID || messages[0].Body != "先に別の方と決まってしまいました。" {
		t.Errorf("交換のメッセージ = (%+v, %v)、申し込まれた人の前後の空白を除いたひとことの1通を期待", messages, err)
	}
}

// TestDeclineTradeUsecase_Execute_WithoutNote は、ひとことが無ければメッセージを残さず、同意が無くてもお断りできることを検証する。
func TestDeclineTradeUsecase_Execute_WithoutNote(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newDeclineTradeUsecase()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	tradeID := testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), receiverID).Build()

	if err := uc.Execute(t.Context(), usecase.DeclineTradeInput{UserID: receiverID, TradeID: tradeID, Reason: "other", Note: " \n "}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
	if err != nil || trade.Status != model.TradeStatusDeclined {
		t.Errorf("お断りしたあとの交換 = (%+v, %v)、お断りを期待", trade, err)
	}
	if count := countTradeRows(t, "trade_messages", tradeID); count != 0 {
		t.Errorf("交換のメッセージの数 = %d、期待値 = 0", count)
	}
}

// TestDeclineTradeUsecase_Execute_Rejected は、お断りできないときに段階を変えず、出来事もメッセージも記録しないことを検証する。
// 返事待ちでない交換では、入力の誤りや同意より先にそのことを返す。
func TestDeclineTradeUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newDeclineTradeUsecase()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	noConsentID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, noConsentID).WithWithdrawnAt(time.Now()).Build()
	pending := testutil.NewTradeBuilder(t, db, proposerID, receiverID).Build()
	matched := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()
	toNoConsent := testutil.NewTradeBuilder(t, db, proposerID, noConsentID).Build()

	tests := []struct {
		name     string
		input    usecase.DeclineTradeInput
		wantCode model.AppErrorCode
	}{
		{name: "無い交換", input: usecase.DeclineTradeInput{UserID: receiverID, TradeID: model.TradeID(uuid.New()), Reason: "other"}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "交換の2人以外", input: usecase.DeclineTradeInput{UserID: testutil.NewUserBuilder(t, db).Build(), TradeID: pending, Reason: "other"}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "申し込んだ人", input: usecase.DeclineTradeInput{UserID: proposerID, TradeID: pending, Reason: "other"}, wantCode: model.AppErrCodeForbidden},
		{name: "返事待ちでなくなった交換", input: usecase.DeclineTradeInput{UserID: receiverID, TradeID: matched}, wantCode: model.AppErrCodeConflict},
		{name: "同意の無い人のひとこと", input: usecase.DeclineTradeInput{UserID: noConsentID, TradeID: toNoConsent, Reason: "other", Note: "ごめんなさい"}, wantCode: model.AppErrCodeMessageConsentRequired},
	}
	for _, tt := range tests {
		err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != tt.wantCode {
			t.Errorf("%s: エラー = %v、コード %d を期待", tt.name, err, tt.wantCode)
		}
	}

	err := uc.Execute(t.Context(), usecase.DeclineTradeInput{UserID: receiverID, TradeID: pending, Reason: "rude"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("reason") {
		t.Errorf("選択肢に無い理由: エラー = %v、理由の欄のエラーを期待", err)
	}

	for _, tradeID := range []model.TradeID{pending, matched, toNoConsent} {
		if count := countTradeRows(t, "trade_events", tradeID) + countTradeRows(t, "trade_messages", tradeID); count != 0 {
			t.Errorf("交換 %s の出来事とメッセージの数 = %d、期待値 = 0", tradeID, count)
		}
	}
	for _, tradeID := range []model.TradeID{pending, toNoConsent} {
		trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
		if err != nil || trade.Status != model.TradeStatusPending {
			t.Errorf("お断りしなかった交換 = (%+v, %v)、返事待ちのままを期待", trade, err)
		}
	}
}
