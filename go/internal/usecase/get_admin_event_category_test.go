package usecase_test

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newGetAdminEventCategoryUsecase はテストのトランザクションの中で動く GetAdminEventCategoryUsecase を作る。
func newGetAdminEventCategoryUsecase(db *sql.DB, tx *sql.Tx) *usecase.GetAdminEventCategoryUsecase {
	return usecase.NewGetAdminEventCategoryUsecase(
		repository.NewEventRepository(db).WithTx(tx),
		repository.NewEventCategoryRepository(db).WithTx(tx),
		repository.NewGoodsRepository(db).WithTx(tx),
	)
}

// TestGetAdminEventCategoryUsecase_Execute は、カテゴリーとそのイベント、削除していないグッズを並び順に返し、
// 削除できるかを管理者だけtrueにすることを検証する。
func TestGetAdminEventCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetAdminEventCategoryUsecase(db, tx)
	eventID := testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithArchived("景品が変わったため").Build()
	secondID := testutil.NewGoodsBuilder(t, tx, categoryID).WithPosition(2).Build()
	firstID := testutil.NewGoodsBuilder(t, tx, categoryID).WithPosition(1).WithArchived("景品から外れたため").Build()
	testutil.NewGoodsBuilder(t, tx, categoryID).WithPosition(0).WithDeleted().Build()

	for role, wantCanDelete := range map[model.UserRole]bool{model.UserRoleEditor: false, model.UserRoleAdmin: true} {
		output, err := uc.Execute(t.Context(), usecase.GetAdminEventCategoryInput{User: newUserWithRole(t, tx, role), EventCategoryID: categoryID})
		if err != nil {
			t.Fatalf("%s: Execute()のエラー = %v", role, err)
		}
		if output.EventCategory.ID != categoryID || output.Event.ID != eventID || output.CanDelete != wantCanDelete {
			t.Errorf("%s: 結果 = (%s, %s, CanDelete %v)、(%s, %s, CanDelete %v) を期待", role, output.EventCategory.ID, output.Event.ID, output.CanDelete, categoryID, eventID, wantCanDelete)
		}
		if len(output.Goods) != 2 || output.Goods[0].ID != firstID || output.Goods[1].ID != secondID {
			t.Errorf("%s: グッズ = %v、[%s %s] を期待", role, output.Goods, firstID, secondID)
		}
	}
}

// TestGetAdminEventCategoryUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、
// 削除したカテゴリー・削除したイベントのカテゴリー・無いカテゴリーには AppErrCodeResourceNotFound を返すことを検証する。
func TestGetAdminEventCategoryUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetAdminEventCategoryUsecase(db, tx)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	eventID := testutil.NewEventBuilder(t, tx).Build()

	_, err := uc.Execute(t.Context(), usecase.GetAdminEventCategoryInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventCategoryID: testutil.NewEventCategoryBuilder(t, tx, eventID).Build()})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	for name, id := range map[string]model.EventCategoryID{
		"削除したカテゴリー":      testutil.NewEventCategoryBuilder(t, tx, eventID).WithDeleted().Build(),
		"削除したイベントのカテゴリー": testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).WithDeleted().Build()).Build(),
		"無いカテゴリー":        model.EventCategoryID(uuid.New()),
	} {
		_, err := uc.Execute(t.Context(), usecase.GetAdminEventCategoryInput{User: editor, EventCategoryID: id})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFound を期待", name, err)
		}
	}
}
