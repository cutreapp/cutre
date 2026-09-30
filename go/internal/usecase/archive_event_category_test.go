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

// newArchiveEventCategoryUsecase はテストのトランザクションの中で動く ArchiveEventCategoryUsecase を作る。
func newArchiveEventCategoryUsecase(db *sql.DB, tx *sql.Tx) *usecase.ArchiveEventCategoryUsecase {
	return usecase.NewArchiveEventCategoryUsecase(
		validator.NewEventCategoryArchiveCreateValidator(),
		repository.NewEventRepository(db).WithTx(tx),
		repository.NewEventCategoryRepository(db).WithTx(tx),
	)
}

// TestArchiveEventCategoryUsecase_Execute は、編集者が理由を残してカテゴリーをアーカイブでき、
// 版が違うときと、もう一度アーカイブしようとすると AppErrCodeConflict を返すことを検証する。
func TestArchiveEventCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newArchiveEventCategoryUsecase(db, tx)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	id := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	err := uc.Execute(t.Context(), usecase.ArchiveEventCategoryInput{User: editor, EventCategoryID: id, LockVersion: 1, ArchiveMessage: "版の違う送信"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)

	if err := uc.Execute(t.Context(), usecase.ArchiveEventCategoryInput{User: editor, EventCategoryID: id, ArchiveMessage: "景品が変わったため"}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := repository.NewEventCategoryRepository(db).WithTx(tx).FindByID(t.Context(), id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "景品が変わったため" {
		t.Errorf("アーカイブしたカテゴリー = %+v、理由を残したアーカイブを期待", found)
	}

	// 状態だけで拒むことを確かめるため、今の版を送る。
	err = uc.Execute(t.Context(), usecase.ArchiveEventCategoryInput{User: editor, EventCategoryID: id, LockVersion: 1, ArchiveMessage: "もう一度"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
}

// TestArchiveEventCategoryUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、
// 理由が空のときは *model.ValidationError を返し、カテゴリーを公開中のまま残すことを検証する。
func TestArchiveEventCategoryUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newArchiveEventCategoryUsecase(db, tx)
	id := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	err := uc.Execute(t.Context(), usecase.ArchiveEventCategoryInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventCategoryID: id, ArchiveMessage: "景品が変わったため"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	err = uc.Execute(jaContext(), usecase.ArchiveEventCategoryInput{User: newUserWithRole(t, tx, model.UserRoleEditor), EventCategoryID: id})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("archive_message") {
		t.Errorf("理由が空のエラー = %v、archive_message の ValidationError を期待", err)
	}

	if found, _ := repository.NewEventCategoryRepository(db).WithTx(tx).FindByID(t.Context(), id); found.Status != model.MasterStatusPublished {
		t.Errorf("状態 = %q、公開中のまま残ることを期待", found.Status)
	}
}
