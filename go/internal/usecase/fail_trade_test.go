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

// newFailTradeUsecase は、自分でトランザクションを開く FailTradeUsecase をテスト用のデータベースで組み立てる。
func newFailTradeUsecase() *usecase.FailTradeUsecase {
	db := testutil.GetTestDB()

	return usecase.NewFailTradeUsecase(
		db,
		validator.NewTradeFailureValidator(),
		repository.NewMessageConsentRepository(db),
		repository.NewTradeRepository(db),
		repository.NewTradeEventRepository(db),
		repository.NewTradeMessageRepository(db),
	)
}

// TestFailTradeUsecase_Execute は、交換の2人のどちらかがマッチ成立の交換を「交換できなかった」で終え、理由とともに出来事を記録して、
// 前後の空白を除いたひとことをメッセージとして残すことを検証する。相手だけが「交換できた」を押したあとでも記録できる。
func TestFailTradeUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newFailTradeUsecase()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, proposerID).Build()
	tradeID := testutil.NewTradeBuilder(t, db, proposerID, testutil.NewUserBuilder(t, db).Build()).
		WithStatus(model.TradeStatusMatched).WithReceiverCompletedAt(time.Now()).Build()

	err := uc.Execute(jaContext(), usecase.FailTradeInput{UserID: proposerID, TradeID: tradeID, Reason: "no_show", Note: " 30分待ちましたが、会えませんでした。 "})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
	if err != nil || trade.Status != model.TradeStatusFailed || trade.EndedAt == nil {
		t.Errorf("記録したあとの交換 = (%+v, %v)、終わった日時のある「交換できなかった」を期待", trade, err)
	}
	events, err := repository.NewTradeEventRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(events) != 1 || events[0].Kind != model.TradeEventKindFailed || events[0].ActorUserID != proposerID ||
		events[0].Reason == nil || *events[0].Reason != "no_show" {
		t.Errorf("交換の出来事 = (%+v, %v)、申し込んだ人の「当日会えなかった」の記録の1件を期待", events, err)
	}
	messages, err := repository.NewTradeMessageRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(messages) != 1 || messages[0].SenderUserID != proposerID || messages[0].Body != "30分待ちましたが、会えませんでした。" {
		t.Errorf("交換のメッセージ = (%+v, %v)、申し込んだ人の前後の空白を除いたひとことの1通を期待", messages, err)
	}
}

// TestFailTradeUsecase_Execute_WithoutNote は、ひとことが無ければメッセージを残さず、同意が無くても記録できることを検証する。
func TestFailTradeUsecase_Execute_WithoutNote(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newFailTradeUsecase()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	tradeID := testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), receiverID).WithStatus(model.TradeStatusMatched).Build()

	if err := uc.Execute(jaContext(), usecase.FailTradeInput{UserID: receiverID, TradeID: tradeID, Reason: "other", Note: " \n "}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
	if err != nil || trade.Status != model.TradeStatusFailed {
		t.Errorf("記録したあとの交換 = (%+v, %v)、「交換できなかった」を期待", trade, err)
	}
	if count := countTradeRows(t, "trade_messages", tradeID); count != 0 {
		t.Errorf("交換のメッセージの数 = %d、期待値 = 0", count)
	}
}

// TestFailTradeUsecase_Execute_Rejected は、記録できないときに段階を変えず、出来事もメッセージも記録しないことを検証する。
// マッチ成立でない交換では、入力の誤りや同意より先にそのことを返す。
func TestFailTradeUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newFailTradeUsecase()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	noConsentID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, noConsentID).WithWithdrawnAt(time.Now()).Build()
	matched := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()
	pending := testutil.NewTradeBuilder(t, db, proposerID, receiverID).Build()
	completed := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusCompleted).Build()
	toNoConsent := testutil.NewTradeBuilder(t, db, proposerID, noConsentID).WithStatus(model.TradeStatusMatched).Build()

	tests := []struct {
		name     string
		input    usecase.FailTradeInput
		wantCode model.AppErrorCode
	}{
		{name: "無い交換", input: usecase.FailTradeInput{UserID: receiverID, TradeID: model.TradeID(uuid.New()), Reason: "other"}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "交換の2人以外", input: usecase.FailTradeInput{UserID: testutil.NewUserBuilder(t, db).Build(), TradeID: matched, Reason: "other"}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "返事待ちの交換", input: usecase.FailTradeInput{UserID: receiverID, TradeID: pending}, wantCode: model.AppErrCodeConflict},
		{name: "終わった交換", input: usecase.FailTradeInput{UserID: receiverID, TradeID: completed}, wantCode: model.AppErrCodeConflict},
		{name: "同意の無い人のひとこと", input: usecase.FailTradeInput{UserID: noConsentID, TradeID: toNoConsent, Reason: "other", Note: "ごめんなさい"}, wantCode: model.AppErrCodeMessageConsentRequired},
	}
	for _, tt := range tests {
		err := uc.Execute(jaContext(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != tt.wantCode {
			t.Errorf("%s: エラー = %v、コード %d を期待", tt.name, err, tt.wantCode)
		}
	}

	err := uc.Execute(jaContext(), usecase.FailTradeInput{UserID: receiverID, TradeID: matched, Reason: "place_mismatch"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("reason") {
		t.Errorf("選択肢に無い理由: エラー = %v、理由の欄のエラーを期待", err)
	}

	for _, tradeID := range []model.TradeID{matched, pending, completed, toNoConsent} {
		if count := countTradeRows(t, "trade_events", tradeID) + countTradeRows(t, "trade_messages", tradeID); count != 0 {
			t.Errorf("交換 %s の出来事とメッセージの数 = %d、期待値 = 0", tradeID, count)
		}
	}
	for _, tradeID := range []model.TradeID{matched, toNoConsent} {
		trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
		if err != nil || trade.Status != model.TradeStatusMatched {
			t.Errorf("記録しなかった交換 = (%+v, %v)、マッチ成立のままを期待", trade, err)
		}
	}
}
