package usecase_test

import (
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetMessagesUsecase_Execute は、ユーザーの交換を終わったものも含めて最新のメッセージの新しい順に返し、
// 相手・交換の品・最新のメッセージ・未読の数を交換ごとにまとめて返すことを検証する。
func TestGetMessagesUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetMessagesUsecase(
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewTradeRepository(db).WithTx(tx),
		repository.NewTradeItemRepository(db).WithTx(tx),
		repository.NewTradeMessageRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
	categoryID := newEventCategoryID(t, tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	endedPartnerID := testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()
	inProgress := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	ended := testutil.NewTradeBuilder(t, tx, endedPartnerID, userID).WithStatus(model.TradeStatusCompleted).Build()
	receiveItemID := testutil.NewItemBuilder(t, tx, partnerID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), inProgress, []model.ItemID{receiveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	now := time.Now()
	testutil.NewTradeMessageBuilder(t, tx, ended, endedPartnerID, "ありがとうございました").WithCreatedAt(now.Add(-time.Hour)).Build()
	latestID := testutil.NewTradeMessageBuilder(t, tx, inProgress, partnerID, "東口でお願いします").WithCreatedAt(now).Build()

	output, err := uc.Execute(t.Context(), usecase.GetMessagesInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.Trades) != 2 || output.Trades[0].ID != inProgress || output.Trades[1].ID != ended {
		t.Fatalf("Trades = %+v、[進行中, 終わった交換] を期待", output.Trades)
	}
	if output.Partners[partnerID] == nil || output.Partners[endedPartnerID] == nil {
		t.Errorf("Partners = %v、退会した相手を含む2人を期待", output.Partners)
	}
	if items := output.Items[inProgress]; len(items) != 1 || items[0].ID != receiveItemID {
		t.Errorf("Items = %v、進行中の交換の品の1点を期待", output.Items)
	}
	if latest := output.LatestMessages[inProgress]; latest == nil || latest.ID != latestID || output.LatestMessages[ended] == nil {
		t.Errorf("LatestMessages = %v、2つの交換の最新の1通を期待", output.LatestMessages)
	}
	if output.UnreadCounts[inProgress] != 1 || output.UnreadCounts[ended] != 1 {
		t.Errorf("UnreadCounts = %v、どちらの交換にも未読の1通を期待", output.UnreadCounts)
	}
}
