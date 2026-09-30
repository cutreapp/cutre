package usecase_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetAdminEventUsecase_Execute は、イベントと削除していないカテゴリーを並び順に返し、削除できるかを管理者だけtrueにすることを検証する。
func TestGetAdminEventUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminEventUsecase(repository.NewEventRepository(db).WithTx(tx), repository.NewEventCategoryRepository(db).WithTx(tx))
	eventID := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()
	secondID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(2).WithArchived("景品が変わったため").Build()
	firstID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(1).Build()
	testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(0).WithDeleted().Build()

	for role, wantCanDelete := range map[model.UserRole]bool{model.UserRoleEditor: false, model.UserRoleAdmin: true} {
		output, err := uc.Execute(t.Context(), usecase.GetAdminEventInput{User: newUserWithRole(t, tx, role), EventID: eventID})
		if err != nil {
			t.Fatalf("%s: Execute()のエラー = %v", role, err)
		}
		if output.Event.ID != eventID || output.CanDelete != wantCanDelete {
			t.Errorf("%s: 結果 = (%s, CanDelete %v)、(%s, CanDelete %v) を期待", role, output.Event.ID, output.CanDelete, eventID, wantCanDelete)
		}
		if len(output.EventCategories) != 2 || output.EventCategories[0].ID != firstID || output.EventCategories[1].ID != secondID {
			t.Errorf("%s: カテゴリー = %v、[%s %s] を期待", role, output.EventCategories, firstID, secondID)
		}
	}
}

// TestGetAdminEventUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、
// 削除したイベントと無いイベントには AppErrCodeResourceNotFound を返すことを検証する。
func TestGetAdminEventUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminEventUsecase(repository.NewEventRepository(db).WithTx(tx), repository.NewEventCategoryRepository(db).WithTx(tx))
	editor := newUserWithRole(t, tx, model.UserRoleEditor)

	_, err := uc.Execute(t.Context(), usecase.GetAdminEventInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventID: testutil.NewEventBuilder(t, tx).Build()})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	_, err = uc.Execute(t.Context(), usecase.GetAdminEventInput{User: editor, EventID: testutil.NewEventBuilder(t, tx).WithDeleted().Build()})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)

	_, err = uc.Execute(t.Context(), usecase.GetAdminEventInput{User: editor, EventID: model.EventID(uuid.New())})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)
}
