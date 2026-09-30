package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestUnarchiveGoodsUsecase_Execute は、編集者がアーカイブしたグッズを公開に戻して理由を空にでき、
// 一般のユーザーには AppErrCodeForbidden を、公開中のグッズには AppErrCodeConflict を返すことを検証する。
func TestUnarchiveGoodsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewUnarchiveGoodsUsecase(eventRepo, categoryRepo, goodsRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	id := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).WithArchived("景品から外れたため").Build()

	err := uc.Execute(t.Context(), usecase.UnarchiveGoodsInput{User: newUserWithRole(t, tx, model.UserRoleUser), GoodsID: id})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	if err := uc.Execute(t.Context(), usecase.UnarchiveGoodsInput{User: editor, GoodsID: id}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := goodsRepo.FindByID(t.Context(), id)
	if found.Status != model.MasterStatusPublished || found.ArchiveMessage != nil {
		t.Errorf("元に戻したグッズ = %+v、公開中・理由なしを期待", found)
	}

	err = uc.Execute(t.Context(), usecase.UnarchiveGoodsInput{User: editor, GoodsID: id, LockVersion: 1})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
}
