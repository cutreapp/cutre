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

// newCreateEventCategoryUsecase はテストのトランザクションの中で動く CreateEventCategoryUsecase を作る。
func newCreateEventCategoryUsecase(db *sql.DB, tx *sql.Tx) *usecase.CreateEventCategoryUsecase {
	return usecase.NewCreateEventCategoryUsecase(
		validator.NewEventCategoryCreateValidator(),
		repository.NewEventRepository(db).WithTx(tx),
		repository.NewEventCategoryRepository(db).WithTx(tx),
	)
}

// TestCreateEventCategoryUsecase_Execute は、編集者がアーカイブしたイベントにも公開中のカテゴリーを作れることを検証する。
func TestCreateEventCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventID := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()

	output, err := newCreateEventCategoryUsecase(db, tx).Execute(t.Context(), usecase.CreateEventCategoryInput{
		User:     newUserWithRole(t, tx, model.UserRoleEditor),
		EventID:  eventID,
		Name:     " A賞 ",
		Position: "1",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	category := output.EventCategory
	if category.EventID != eventID || category.Name != "A賞" || category.Position != 1 || category.Status != model.MasterStatusPublished {
		t.Errorf("作ったカテゴリー = %+v、イベント %s の配下に公開中の「A賞」・並び順1を期待", category, eventID)
	}
}

// TestCreateEventCategoryUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、削除したイベントには
// AppErrCodeResourceNotFound を、フォームの誤りには *model.ValidationError を返し、カテゴリーを作らないことを検証する。
func TestCreateEventCategoryUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newCreateEventCategoryUsecase(db, tx)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	eventID := testutil.NewEventBuilder(t, tx).Build()

	_, err := uc.Execute(t.Context(), usecase.CreateEventCategoryInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventID: eventID, Name: "A賞", Position: "1"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	_, err = uc.Execute(t.Context(), usecase.CreateEventCategoryInput{User: editor, EventID: testutil.NewEventBuilder(t, tx).WithDeleted().Build(), Name: "A賞", Position: "1"})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)

	_, err = uc.Execute(jaContext(), usecase.CreateEventCategoryInput{User: editor, EventID: eventID, Name: "A賞", Position: "x"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("position") {
		t.Errorf("並び順が読めないエラー = %v、position の ValidationError を期待", err)
	}

	categories, err := repository.NewEventCategoryRepository(db).WithTx(tx).ListUndeletedByEventID(t.Context(), eventID)
	if err != nil || len(categories) != 0 {
		t.Errorf("イベントのカテゴリー = (%v, %v)、カテゴリーを作らないことを期待", categories, err)
	}
}
