package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestCreateGoodsUsecase_Execute は、編集者がアーカイブしたカテゴリーにも公開中のグッズを作れることを検証する。
func TestCreateGoodsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewCreateGoodsUsecase(validator.NewGoodsCreateValidator(), eventRepo, categoryRepo, goodsRepo)
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).WithArchived("景品が変わったため").Build()

	output, err := uc.Execute(t.Context(), usecase.CreateGoodsInput{
		User:            newUserWithRole(t, tx, model.UserRoleEditor),
		EventCategoryID: categoryID,
		Name:            "くまの子",
		Position:        "1",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	goods := output.Goods
	if goods.EventCategoryID != categoryID || goods.Name != "くまの子" || goods.Position != 1 || goods.Status != model.MasterStatusPublished {
		t.Errorf("作ったグッズ = %+v、カテゴリー %s の配下に公開中の「くまの子」・並び順1を期待", goods, categoryID)
	}
}

// TestCreateGoodsUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、削除したカテゴリーには
// AppErrCodeResourceNotFound を、フォームの誤りには *model.ValidationError を返し、グッズを作らないことを検証する。
func TestCreateGoodsUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewCreateGoodsUsecase(validator.NewGoodsCreateValidator(), eventRepo, categoryRepo, goodsRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	categoryID := newEventCategoryID(t, tx)

	_, err := uc.Execute(t.Context(), usecase.CreateGoodsInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventCategoryID: categoryID, Name: "くまの子", Position: "1"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	deletedCategoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).WithDeleted().Build()
	_, err = uc.Execute(t.Context(), usecase.CreateGoodsInput{User: editor, EventCategoryID: deletedCategoryID, Name: "くまの子", Position: "1"})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)

	_, err = uc.Execute(jaContext(), usecase.CreateGoodsInput{User: editor, EventCategoryID: categoryID, Name: "", Position: "1"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("name") {
		t.Errorf("名前が空のエラー = %v、name の ValidationError を期待", err)
	}

	goods, err := goodsRepo.ListUndeletedByEventCategoryID(t.Context(), categoryID)
	if err != nil || len(goods) != 0 {
		t.Errorf("カテゴリーのグッズ = (%v, %v)、グッズを作らないことを期待", goods, err)
	}
}
