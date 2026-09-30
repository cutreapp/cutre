package usecase_test

import (
	"database/sql"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

func newGetEventCategoryUsecase(db *sql.DB, tx *sql.Tx) *usecase.GetEventCategoryUsecase {
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	return usecase.NewGetEventCategoryUsecase(eventRepo, categoryRepo, goodsRepo, repository.NewItemRepository(db).WithTx(tx))
}

// TestGetEventCategoryUsecase_Execute は、公開中のグッズと、ユーザーのリストにあるアイテムを返すことを検証する。
func TestGetEventCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetEventCategoryUsecase(db, tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	testutil.NewGoodsBuilder(t, tx, categoryID).WithArchived("景品から外れたため").Build()
	itemID := testutil.NewItemBuilder(t, tx, userID, goodsID).Build()
	testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).Build()

	output, err := uc.Execute(t.Context(), usecase.GetEventCategoryInput{UserID: userID, EventID: eventID, EventCategoryID: categoryID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Event.ID != eventID || output.EventCategory.ID != categoryID {
		t.Errorf("イベントとカテゴリー = (%s, %s)、(%s, %s) を期待", output.Event.ID, output.EventCategory.ID, eventID, categoryID)
	}
	if len(output.Goods) != 1 || output.Goods[0].ID != goodsID {
		t.Errorf("グッズ = %+v、公開中の %s だけを期待", output.Goods, goodsID)
	}
	if len(output.Items) != 1 || output.Items[0].ID != itemID {
		t.Errorf("アイテム = %+v、ユーザーの %s だけを期待", output.Items, itemID)
	}
}

// TestGetEventCategoryUsecase_Execute_NotFound は、カテゴリーかイベントを公開していないときと、
// カテゴリーがURLのイベントのものでないときに AppErrCodeResourceNotFound を返すことを検証する。
func TestGetEventCategoryUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetEventCategoryUsecase(db, tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	archivedEventID := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()

	tests := []struct {
		name  string
		input usecase.GetEventCategoryInput
	}{
		{name: "アーカイブしたカテゴリー", input: usecase.GetEventCategoryInput{EventID: eventID, EventCategoryID: testutil.NewEventCategoryBuilder(t, tx, eventID).WithArchived("景品から外れたため").Build()}},
		{name: "アーカイブしたイベントのカテゴリー", input: usecase.GetEventCategoryInput{EventID: archivedEventID, EventCategoryID: testutil.NewEventCategoryBuilder(t, tx, archivedEventID).Build()}},
		{name: "ほかのイベントのカテゴリー", input: usecase.GetEventCategoryInput{EventID: eventID, EventCategoryID: testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()}},
	}
	for _, tt := range tests {
		tt.input.UserID = userID
		_, err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFound を期待", tt.name, err)
		}
	}
}
