package usecase_test

import (
	"slices"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetAdminEventsUsecase_Execute は、編集者に公開中とアーカイブしたイベントを返し、削除したイベントを含めないことを検証する。
func TestGetAdminEventsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminEventsUsecase(repository.NewEventRepository(db).WithTx(tx))
	publishedID := testutil.NewEventBuilder(t, tx).Build()
	archivedID := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()
	deletedID := testutil.NewEventBuilder(t, tx).WithDeleted().Build()

	output, err := uc.Execute(t.Context(), usecase.GetAdminEventsInput{User: newUserWithRole(t, tx, model.UserRoleEditor)})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	ids := make([]model.EventID, len(output.Events))
	for i, event := range output.Events {
		ids[i] = event.ID
	}
	if !slices.Contains(ids, publishedID) || !slices.Contains(ids, archivedID) || slices.Contains(ids, deletedID) {
		t.Errorf("一覧のイベント = %v、公開中 %s とアーカイブ %s を含み、削除 %s を含まないことを期待", ids, publishedID, archivedID, deletedID)
	}
}

// TestGetAdminEventsUsecase_Execute_Forbidden は、一般のユーザーに AppErrCodeForbidden を返すことを検証する。
func TestGetAdminEventsUsecase_Execute_Forbidden(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminEventsUsecase(repository.NewEventRepository(db).WithTx(tx))

	_, err := uc.Execute(t.Context(), usecase.GetAdminEventsInput{User: newUserWithRole(t, tx, model.UserRoleUser)})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)
}
