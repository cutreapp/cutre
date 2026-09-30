package usecase_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newCreateTradeMessageUsecase はテストのトランザクションで読み書きする CreateTradeMessageUsecase を組み立てる。
func newCreateTradeMessageUsecase(db *sql.DB, tx *sql.Tx) *usecase.CreateTradeMessageUsecase {
	return usecase.NewCreateTradeMessageUsecase(
		validator.NewTradeMessageCreateValidator(),
		repository.NewMessageConsentRepository(db).WithTx(tx),
		repository.NewTradeRepository(db).WithTx(tx),
		repository.NewTradeMessageRepository(db).WithTx(tx),
	)
}

// countTradeMessages は交換 tradeID のメッセージの数を返す。
func countTradeMessages(t *testing.T, db *sql.DB, tx *sql.Tx, tradeID model.TradeID) int {
	t.Helper()

	messages, err := repository.NewTradeMessageRepository(db).WithTx(tx).ListByTradeID(t.Context(), tradeID)
	if err != nil {
		t.Fatalf("ListByTradeID()のエラー = %v", err)
	}

	return len(messages)
}

// TestCreateTradeMessageUsecase_Execute は、返事待ちとマッチ成立の交換で、交換の2人のどちらからも、
// 前後の空白を除いた本文のメッセージを送れることを検証する。
func TestCreateTradeMessageUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newCreateTradeMessageUsecase(db, tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewMessageConsentBuilder(t, tx, proposerID).Build()
	testutil.NewMessageConsentBuilder(t, tx, receiverID).Build()

	tests := []struct {
		name     string
		status   model.TradeStatus
		senderID model.UserID
	}{
		{name: "返事待ちの申し込んだ人", status: model.TradeStatusPending, senderID: proposerID},
		{name: "返事待ちの申し込まれた人", status: model.TradeStatusPending, senderID: receiverID},
		{name: "マッチ成立", status: model.TradeStatusMatched, senderID: proposerID},
	}
	for _, tt := range tests {
		tradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(tt.status).Build()

		output, err := uc.Execute(t.Context(), usecase.CreateTradeMessageInput{SenderUserID: tt.senderID, TradeID: tradeID, Body: " 土曜の13時に\n新宿駅はどうでしょう？\n"})
		if err != nil {
			t.Fatalf("%s: Execute()のエラー = %v", tt.name, err)
		}
		if output.Message.TradeID != tradeID || output.Message.SenderUserID != tt.senderID || output.Message.Body != "土曜の13時に\n新宿駅はどうでしょう？" {
			t.Errorf("%s: 送ったメッセージ = %+v、交換・送った人・前後の空白を除いた本文を期待", tt.name, output.Message)
		}
	}
}

// TestCreateTradeMessageUsecase_Execute_Rejected は、送れないときにそれぞれのエラーを返し、メッセージを記録しないことを検証する。
func TestCreateTradeMessageUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newCreateTradeMessageUsecase(db, tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewMessageConsentBuilder(t, tx, proposerID).Build()
	testutil.NewMessageConsentBuilder(t, tx, receiverID).WithWithdrawnAt(time.Now()).Build()
	pendingID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	completedID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusCompleted).Build()

	tests := []struct {
		name     string
		input    usecase.CreateTradeMessageInput
		wantCode model.AppErrorCode
	}{
		{
			name:     "交換の2人以外",
			input:    usecase.CreateTradeMessageInput{SenderUserID: testutil.NewUserBuilder(t, tx).Build(), TradeID: pendingID, Body: "本文"},
			wantCode: model.AppErrCodeResourceNotFound,
		},
		{
			name:     "無い交換",
			input:    usecase.CreateTradeMessageInput{SenderUserID: proposerID, TradeID: model.TradeID(uuid.New()), Body: "本文"},
			wantCode: model.AppErrCodeResourceNotFound,
		},
		{
			name:     "終わった交換",
			input:    usecase.CreateTradeMessageInput{SenderUserID: proposerID, TradeID: completedID, Body: "本文"},
			wantCode: model.AppErrCodeConflict,
		},
		{
			name:     "同意をやめた人",
			input:    usecase.CreateTradeMessageInput{SenderUserID: receiverID, TradeID: pendingID, Body: "本文"},
			wantCode: model.AppErrCodeMessageConsentRequired,
		},
	}
	for _, tt := range tests {
		_, err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != tt.wantCode {
			t.Errorf("%s: Execute()のエラー = %v、%v を期待", tt.name, err, tt.wantCode)
		}
	}

	_, err := uc.Execute(t.Context(), usecase.CreateTradeMessageInput{SenderUserID: proposerID, TradeID: pendingID, Body: " \n "})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("body") {
		t.Errorf("空の本文のExecute()のエラー = %v、body のエラーを期待", err)
	}

	if n := countTradeMessages(t, db, tx, pendingID) + countTradeMessages(t, db, tx, completedID); n != 0 {
		t.Errorf("記録したメッセージの数 = %d、0を期待", n)
	}
}
