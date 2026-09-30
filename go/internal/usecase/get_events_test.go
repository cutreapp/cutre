package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetEventsUsecase_Execute は、公開中のイベントを返し、イベントごとのグッズの数と
// ユーザーのアイテムの数量を添えることを検証する。
func TestGetEventsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, _, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewGetEventsUsecase(eventRepo, goodsRepo, repository.NewItemRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	archivedEventID := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, eventID).Build()).Build()
	testutil.NewItemBuilder(t, tx, userID, goodsID).WithQuantity(2).Build()

	output, err := uc.Execute(t.Context(), usecase.GetEventsInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	var found bool
	for _, event := range output.Events {
		if event.ID == archivedEventID {
			t.Error("アーカイブしたイベントを一覧に含めた")
		}
		found = found || event.ID == eventID
	}
	if !found {
		t.Errorf("一覧に公開中のイベント %s が無い", eventID)
	}
	if output.GoodsCounts[eventID] != 1 || output.Quantities[eventID] != (model.ItemQuantities{Give: 2}) {
		t.Errorf("グッズの数と数量 = (%d, %+v)、(1, 譲れる2) を期待", output.GoodsCounts[eventID], output.Quantities[eventID])
	}
}
