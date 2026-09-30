package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestUpdateGoodsUsecase_Execute は、編集者がグッズを更新して戻り先のカテゴリーのIDを受け取れ、古い版からの更新には
// AppErrCodeConflict を、一般のユーザーには AppErrCodeForbidden を返して上書きしないことを検証する。
func TestUpdateGoodsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewUpdateGoodsUsecase(validator.NewGoodsUpdateValidator(), eventRepo, categoryRepo, goodsRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	categoryID := newEventCategoryID(t, tx)
	id := testutil.NewGoodsBuilder(t, tx, categoryID).Build()

	_, err := uc.Execute(t.Context(), usecase.UpdateGoodsInput{User: newUserWithRole(t, tx, model.UserRoleUser), GoodsID: id, Name: "うさぎの子", Position: "2"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	output, err := uc.Execute(t.Context(), usecase.UpdateGoodsInput{User: editor, GoodsID: id, Name: "うさぎの子", Position: "2"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.EventCategoryID != categoryID {
		t.Errorf("EventCategoryID = %s、期待値 = %s", output.EventCategoryID, categoryID)
	}

	_, err = uc.Execute(t.Context(), usecase.UpdateGoodsInput{User: editor, GoodsID: id, Name: "古い版からのグッズ", Position: "3"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)

	found, _ := goodsRepo.FindByID(t.Context(), id)
	if found.Name != "うさぎの子" || found.Position != 2 || found.LockVersion != 1 {
		t.Errorf("グッズ = %+v、「うさぎの子」・並び順2・版1を期待", found)
	}
}
