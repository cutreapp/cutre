package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetMainNavUsecase_Execute は、ユーザーの返事を待っている交換の数と、未読のメッセージの数を返すことを検証する。
func TestGetMainNavUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetMainNavUsecase(repository.NewTradeRepository(db).WithTx(tx), repository.NewTradeMessageRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	awaitingTradeID := testutil.NewTradeBuilder(t, tx, partnerID, userID).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	testutil.NewTradeMessageBuilder(t, tx, awaitingTradeID, partnerID, "はじめまして").Build()
	testutil.NewTradeMessageBuilder(t, tx, awaitingTradeID, userID, "よろしくお願いします").Build()

	output, err := uc.Execute(t.Context(), usecase.GetMainNavInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.AwaitingTradeCount != 1 {
		t.Errorf("AwaitingTradeCount = %d、期待値 = 1", output.AwaitingTradeCount)
	}
	if output.UnreadMessageCount != 1 {
		t.Errorf("UnreadMessageCount = %d、期待値 = 1", output.UnreadMessageCount)
	}
}
