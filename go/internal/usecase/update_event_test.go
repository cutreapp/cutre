package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestUpdateEventUsecase_Execute は、フォームを開いたときの版で更新して版を上げ、
// 同じ古い版からもう一度送ると上書きせずに AppErrCodeConflict を返すことを検証する。
func TestUpdateEventUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	uc := usecase.NewUpdateEventUsecase(validator.NewEventUpdateValidator(), repo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	eventID := testutil.NewEventBuilder(t, tx).Build()
	input := usecase.UpdateEventInput{User: editor, EventID: eventID, LockVersion: 0, Name: "冬のくじ", StartsOn: "2026-12-01", EndsOn: "2026-12-31"}

	if err := uc.Execute(t.Context(), input); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := repo.FindByID(t.Context(), eventID)
	if found.Name != "冬のくじ" || found.EndsOn == nil || found.LockVersion != 1 {
		t.Errorf("更新したイベント = %+v、「冬のくじ」・終了日あり・版1を期待", found)
	}

	input.Name = "古い版からの名前"
	assertAppErrorCode(t, uc.Execute(t.Context(), input), model.AppErrCodeConflict)
	found, _ = repo.FindByID(t.Context(), eventID)
	if found.Name != "冬のくじ" {
		t.Errorf("古い版から送った後の名前 = %q、上書きされないことを期待", found.Name)
	}
}

// TestUpdateEventUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、削除したイベントには
// AppErrCodeResourceNotFound を、フォームの誤りには *model.ValidationError を返すことを検証する。
func TestUpdateEventUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewUpdateEventUsecase(validator.NewEventUpdateValidator(), repository.NewEventRepository(db).WithTx(tx))
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	eventID := testutil.NewEventBuilder(t, tx).Build()

	err := uc.Execute(t.Context(), usecase.UpdateEventInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventID: eventID, Name: "冬のくじ", StartsOn: "2026-12-01"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	err = uc.Execute(t.Context(), usecase.UpdateEventInput{User: editor, EventID: testutil.NewEventBuilder(t, tx).WithDeleted().Build(), Name: "冬のくじ", StartsOn: "2026-12-01"})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)

	err = uc.Execute(jaContext(), usecase.UpdateEventInput{User: editor, EventID: eventID, Name: "冬のくじ", StartsOn: "2026-12-01", EndsOn: "2026-11-30"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("ends_on") {
		t.Errorf("開始日より前の終了日のエラー = %v、ends_on の ValidationError を期待", err)
	}
}
