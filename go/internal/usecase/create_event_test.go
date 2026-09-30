package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestCreateEventUsecase_Execute は、編集者がフォームの値から公開中のイベントを作成できることを検証する。
func TestCreateEventUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	uc := usecase.NewCreateEventUsecase(validator.NewEventCreateValidator(), repo)

	output, err := uc.Execute(t.Context(), usecase.CreateEventInput{
		User:     newUserWithRole(t, tx, model.UserRoleEditor),
		Name:     "秋のくじ",
		StartsOn: "2026-10-01",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, err := repo.FindByID(t.Context(), output.Event.ID)
	if err != nil || found == nil || found.Name != "秋のくじ" || found.Status != model.MasterStatusPublished || found.EndsOn != nil {
		t.Errorf("作成したイベント = (%+v, %v)、公開中で終わりの決まっていない「秋のくじ」を期待", found, err)
	}
}

// TestCreateEventUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、
// フォームの誤りには *model.ValidationError を返すことを検証する。
func TestCreateEventUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewCreateEventUsecase(validator.NewEventCreateValidator(), repository.NewEventRepository(db).WithTx(tx))

	_, err := uc.Execute(t.Context(), usecase.CreateEventInput{User: newUserWithRole(t, tx, model.UserRoleUser), Name: "秋のくじ", StartsOn: "2026-10-01"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	_, err = uc.Execute(jaContext(), usecase.CreateEventInput{User: newUserWithRole(t, tx, model.UserRoleEditor), StartsOn: "2026-10-01"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("name") {
		t.Errorf("名前が空のエラー = %v、name の ValidationError を期待", err)
	}
}
