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

// TestMarkTradeMessagesReadUsecase_Execute は、交換の2人が読んだところを記録し、交換が無いときと交換の2人以外には
// AppErrCodeResourceNotFound を返して記録しないことを検証する。
func TestMarkTradeMessagesReadUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	readRepo := repository.NewTradeMessageReadRepository(db).WithTx(tx)
	uc := usecase.NewMarkTradeMessagesReadUsecase(repository.NewTradeRepository(db).WithTx(tx), readRepo)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, testutil.NewUserBuilder(t, tx).Build()).Build()
	readAt := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	messageID := testutil.NewTradeMessageBuilder(t, tx, tradeID, proposerID, "読んだメッセージ").WithCreatedAt(readAt).Build()
	position := model.TradeMessageReadPosition{CreatedAt: readAt, MessageID: messageID}

	if err := uc.Execute(t.Context(), usecase.MarkTradeMessagesReadInput{UserID: proposerID, TradeID: tradeID, LastReadPosition: position}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if got, err := readRepo.FindLastReadPosition(t.Context(), tradeID, proposerID); err != nil || got == nil || !got.CreatedAt.Equal(readAt) || got.MessageID != messageID {
		t.Errorf("FindLastReadPosition() = (%v, %v)、%v を期待", got, err, position)
	}

	outsiderID := testutil.NewUserBuilder(t, tx).Build()
	tests := []struct {
		name  string
		input usecase.MarkTradeMessagesReadInput
	}{
		{name: "交換の2人以外", input: usecase.MarkTradeMessagesReadInput{UserID: outsiderID, TradeID: tradeID, LastReadPosition: position}},
		{name: "無い交換", input: usecase.MarkTradeMessagesReadInput{UserID: proposerID, TradeID: model.TradeID(uuid.New()), LastReadPosition: position}},
	}
	for _, tt := range tests {
		err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: Execute()のエラー = %v、AppErrCodeResourceNotFound を期待", tt.name, err)
		}
	}
	if got, err := readRepo.FindLastReadPosition(t.Context(), tradeID, outsiderID); err != nil || got != nil {
		t.Errorf("交換の2人以外: FindLastReadPosition() = (%v, %v)、記録しないことを期待", got, err)
	}
}
