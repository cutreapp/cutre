package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestArchiveGoodsUsecase_Execute は、編集者が理由を残してグッズをアーカイブでき、理由が空のときは
// *model.ValidationError を、もう一度アーカイブしようとすると AppErrCodeConflict を返すことを検証する。
func TestArchiveGoodsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewArchiveGoodsUsecase(validator.NewGoodsArchiveCreateValidator(), eventRepo, categoryRepo, goodsRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	id := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()

	err := uc.Execute(jaContext(), usecase.ArchiveGoodsInput{User: editor, GoodsID: id})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("archive_message") {
		t.Errorf("理由が空のエラー = %v、archive_message の ValidationError を期待", err)
	}

	if err := uc.Execute(t.Context(), usecase.ArchiveGoodsInput{User: editor, GoodsID: id, ArchiveMessage: "景品から外れたため"}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := goodsRepo.FindByID(t.Context(), id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "景品から外れたため" {
		t.Errorf("アーカイブしたグッズ = %+v、理由を残したアーカイブを期待", found)
	}

	// 状態だけで拒むことを確かめるため、今の版を送る。
	err = uc.Execute(t.Context(), usecase.ArchiveGoodsInput{User: editor, GoodsID: id, LockVersion: 1, ArchiveMessage: "もう一度"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
}
