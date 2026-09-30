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
)

// newRetractTradeMessageUsecase はテストのトランザクションで読み書きする RetractTradeMessageUsecase を組み立てる。
func newRetractTradeMessageUsecase(db *sql.DB, tx *sql.Tx) *usecase.RetractTradeMessageUsecase {
	return usecase.NewRetractTradeMessageUsecase(repository.NewTradeRepository(db).WithTx(tx), repository.NewTradeMessageRepository(db).WithTx(tx))
}

// TestRetractTradeMessageUsecase_Execute は、送った人が、終わった交換でもメッセージを取り消せ、本文を残すことと、
// 取り消し済みのメッセージをもう一度取り消してもエラーにしないことを検証する。
func TestRetractTradeMessageUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newRetractTradeMessageUsecase(db, tx)
	senderID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, senderID, testutil.NewUserBuilder(t, tx).Build()).WithStatus(model.TradeStatusCompleted).Build()
	messageID := testutil.NewTradeMessageBuilder(t, tx, tradeID, senderID, "取り消す本文").Build()
	input := usecase.RetractTradeMessageInput{UserID: senderID, TradeID: tradeID, MessageID: messageID}

	if err := uc.Execute(t.Context(), input); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	message, err := repository.NewTradeMessageRepository(db).WithTx(tx).FindByID(t.Context(), messageID)
	if err != nil || message.RetractedAt == nil || message.Body != "取り消す本文" {
		t.Errorf("取り消したメッセージ = (%+v, %v)、本文を残した取り消したメッセージを期待", message, err)
	}
	if err := uc.Execute(t.Context(), input); err != nil {
		t.Errorf("取り消し済み: Execute()のエラー = %v、nilを期待", err)
	}
}

// TestRetractTradeMessageUsecase_Execute_Rejected は、取り消せないときにエラーを返し、メッセージを取り消さないことを検証する。
func TestRetractTradeMessageUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newRetractTradeMessageUsecase(db, tx)
	senderID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, senderID, partnerID).Build()
	otherTradeID := testutil.NewTradeBuilder(t, tx, senderID, partnerID).Build()
	messageID := testutil.NewTradeMessageBuilder(t, tx, tradeID, senderID, "取り消さない本文").WithCreatedAt(time.Now()).Build()

	tests := []struct {
		name  string
		input usecase.RetractTradeMessageInput
		want  model.AppErrorCode
	}{
		{name: "交換の相手", input: usecase.RetractTradeMessageInput{UserID: partnerID, TradeID: tradeID, MessageID: messageID}, want: model.AppErrCodeForbidden},
		{name: "交換の2人以外", input: usecase.RetractTradeMessageInput{UserID: testutil.NewUserBuilder(t, tx).Build(), TradeID: tradeID, MessageID: messageID}, want: model.AppErrCodeResourceNotFound},
		{name: "別の交換のパス", input: usecase.RetractTradeMessageInput{UserID: senderID, TradeID: otherTradeID, MessageID: messageID}, want: model.AppErrCodeResourceNotFound},
		{name: "無い交換", input: usecase.RetractTradeMessageInput{UserID: senderID, TradeID: model.TradeID(uuid.New()), MessageID: messageID}, want: model.AppErrCodeResourceNotFound},
		{name: "無いメッセージ", input: usecase.RetractTradeMessageInput{UserID: senderID, TradeID: tradeID, MessageID: model.TradeMessageID(uuid.New())}, want: model.AppErrCodeResourceNotFound},
	}
	for _, tt := range tests {
		err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != tt.want {
			t.Errorf("%s: Execute()のエラー = %v、%v を期待", tt.name, err, tt.want)
		}
	}

	message, err := repository.NewTradeMessageRepository(db).WithTx(tx).FindByID(t.Context(), messageID)
	if err != nil || message.RetractedAt != nil {
		t.Errorf("メッセージ = (%+v, %v)、取り消していないことを期待", message, err)
	}
}
