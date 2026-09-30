package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetWithdrawalUsecase_Execute は、ユーザーの進行中の交換の数を返し、終わった交換は数えないことを検証する。
func TestGetWithdrawalUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetWithdrawalUsecase(repository.NewTradeRepository(db).WithTx(tx))

	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusMatched).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusCompleted).Build()

	output, err := uc.Execute(t.Context(), usecase.GetWithdrawalInput{UserID: userID})
	if err != nil || output.InProgressTradeCount != 2 {
		t.Errorf("Execute() = (%+v, %v)、進行中の交換の数2を期待", output, err)
	}
}
