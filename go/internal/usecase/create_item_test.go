package usecase_test

import (
	"database/sql"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newCreateItemUsecase はテスト用のデータベースに直接書き込む CreateItemUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newCreateItemUsecase(db *sql.DB) *usecase.CreateItemUsecase {
	return usecase.NewCreateItemUsecase(db, validator.NewItemCreateValidator(), repository.NewEventRepository(db), repository.NewEventCategoryRepository(db), repository.NewGoodsRepository(db), repository.NewItemRepository(db))
}

// TestCreateItemUsecase_Execute は、公開中のグッズのアイテムをリストに入れ、戻り先のカテゴリーとイベントのIDを返すことと、
// 同じリストに2つ目を入れようとすると AppErrCodeConflict を返すことを検証する。
func TestCreateItemUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newCreateItemUsecase(db)
	userID := testutil.NewUserBuilder(t, db).Build()
	eventID := testutil.NewEventBuilder(t, db).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, db, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, db, categoryID).Build()
	input := usecase.CreateItemInput{UserID: userID, GoodsID: goodsID, Kind: "give", Quantity: "2", Note: "未開封"}

	output, err := uc.Execute(jaContext(), input)
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Item.UserID != userID || output.Item.GoodsID != goodsID || output.Item.Kind != model.ItemKindGive || output.Item.Quantity != 2 {
		t.Errorf("アイテム = %+v、ユーザーの譲れるリストに2点を期待", output.Item)
	}
	if output.EventID != eventID || output.EventCategoryID != categoryID {
		t.Errorf("戻り先 = (%s, %s)、(%s, %s) を期待", output.EventID, output.EventCategoryID, eventID, categoryID)
	}

	_, err = uc.Execute(jaContext(), usecase.CreateItemInput{UserID: userID, GoodsID: goodsID, Kind: "give", Quantity: "0"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("quantity") {
		t.Errorf("数量0のエラー = %v、quantity の ValidationError を期待", err)
	}

	_, err = uc.Execute(jaContext(), input)
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
}

// TestCreateItemUsecase_Execute_NotPublished は、公開していないグッズ・カテゴリー・イベントのグッズを
// リストに入れず、AppErrCodeResourceNotFound を返すことを検証する。
func TestCreateItemUsecase_Execute_NotPublished(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newCreateItemUsecase(db)
	userID := testutil.NewUserBuilder(t, db).Build()
	publishedCategoryID := testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()

	goodsIDs := map[string]model.GoodsID{
		"アーカイブしたグッズ":       testutil.NewGoodsBuilder(t, db, publishedCategoryID).WithArchived("景品から外れたため").Build(),
		"アーカイブしたカテゴリーのグッズ": testutil.NewGoodsBuilder(t, db, testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).WithArchived("景品から外れたため").Build()).Build(),
		"削除したイベントのグッズ":     testutil.NewGoodsBuilder(t, db, testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).WithDeleted().Build()).Build()).Build(),
	}
	for name, goodsID := range goodsIDs {
		_, err := uc.Execute(jaContext(), usecase.CreateItemInput{UserID: userID, GoodsID: goodsID, Kind: "give", Quantity: "1"})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFound を期待", name, err)
		}
	}
}
