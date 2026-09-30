package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestArchiveEventUsecase_Execute は、編集者が理由を残してイベントをアーカイブでき、
// 版が違うときと、もう一度アーカイブしようとすると AppErrCodeConflict を返すことを検証する。
func TestArchiveEventUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	uc := usecase.NewArchiveEventUsecase(validator.NewEventArchiveCreateValidator(), repo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	eventID := testutil.NewEventBuilder(t, tx).Build()

	err := uc.Execute(t.Context(), usecase.ArchiveEventInput{User: editor, EventID: eventID, LockVersion: 1, ArchiveMessage: "版の違う送信"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)

	if err := uc.Execute(t.Context(), usecase.ArchiveEventInput{User: editor, EventID: eventID, ArchiveMessage: "開催が終わったため"}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := repo.FindByID(t.Context(), eventID)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "開催が終わったため" {
		t.Errorf("アーカイブしたイベント = %+v、理由を残したアーカイブを期待", found)
	}

	// 状態だけで拒むことを確かめるため、今の版を送る。
	err = uc.Execute(t.Context(), usecase.ArchiveEventInput{User: editor, EventID: eventID, LockVersion: 1, ArchiveMessage: "もう一度"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
}

// TestArchiveEventUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、
// 理由が空のときは *model.ValidationError を返し、イベントを公開中のまま残すことを検証する。
func TestArchiveEventUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	uc := usecase.NewArchiveEventUsecase(validator.NewEventArchiveCreateValidator(), repo)
	eventID := testutil.NewEventBuilder(t, tx).Build()

	err := uc.Execute(t.Context(), usecase.ArchiveEventInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventID: eventID, ArchiveMessage: "開催が終わったため"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	err = uc.Execute(jaContext(), usecase.ArchiveEventInput{User: newUserWithRole(t, tx, model.UserRoleEditor), EventID: eventID})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("archive_message") {
		t.Errorf("理由が空のエラー = %v、archive_message の ValidationError を期待", err)
	}

	if found, _ := repo.FindByID(t.Context(), eventID); found.Status != model.MasterStatusPublished {
		t.Errorf("状態 = %q、公開中のまま残ることを期待", found.Status)
	}
}
