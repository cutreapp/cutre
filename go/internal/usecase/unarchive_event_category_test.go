package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestUnarchiveEventCategoryUsecase_Execute は、編集者がアーカイブしたカテゴリーを公開に戻して理由を空にでき、
// 一般のユーザーには AppErrCodeForbidden を、公開中のカテゴリーには AppErrCodeConflict を返すことを検証する。
func TestUnarchiveEventCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	categoryRepo := repository.NewEventCategoryRepository(db).WithTx(tx)
	uc := usecase.NewUnarchiveEventCategoryUsecase(repository.NewEventRepository(db).WithTx(tx), categoryRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	id := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).WithArchived("景品が変わったため").Build()

	err := uc.Execute(t.Context(), usecase.UnarchiveEventCategoryInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventCategoryID: id})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	if err := uc.Execute(t.Context(), usecase.UnarchiveEventCategoryInput{User: editor, EventCategoryID: id}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := categoryRepo.FindByID(t.Context(), id)
	if found.Status != model.MasterStatusPublished || found.ArchiveMessage != nil {
		t.Errorf("元に戻したカテゴリー = %+v、公開中・理由なしを期待", found)
	}

	err = uc.Execute(t.Context(), usecase.UnarchiveEventCategoryInput{User: editor, EventCategoryID: id, LockVersion: 1})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
}
