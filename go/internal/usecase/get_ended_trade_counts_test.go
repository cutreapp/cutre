package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetEndedTradeCountsUsecase_Execute は、ユーザーの終わった交換の数を段階ごとに返し、進行中の交換を数えないことを検証する。
func TestGetEndedTradeCountsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetEndedTradeCountsUsecase(repository.NewTradeRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusCancelled).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusMatched).Build()

	output, err := uc.Execute(t.Context(), usecase.GetEndedTradeCountsInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.Counts) != 2 || output.Counts[model.TradeStatusCompleted] != 1 || output.Counts[model.TradeStatusCancelled] != 1 {
		t.Errorf("Counts = %v、交換できた1件・やめた1件を期待", output.Counts)
	}
}
