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

// newGoodsRepos はテストのトランザクションの中で動く、グッズのUseCaseが使う3つのリポジトリを作る。
func newGoodsRepos(db *sql.DB, tx *sql.Tx) (*repository.EventRepository, *repository.EventCategoryRepository, *repository.GoodsRepository) {
	return repository.NewEventRepository(db).WithTx(tx), repository.NewEventCategoryRepository(db).WithTx(tx), repository.NewGoodsRepository(db).WithTx(tx)
}

// newEventCategoryID はテスト用のイベントと、その配下のカテゴリーを作り、カテゴリーのIDを返す。
func newEventCategoryID(t *testing.T, tx *sql.Tx) model.EventCategoryID {
	t.Helper()

	return testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()
}

// TestGetAdminGoodsUsecase_Execute は、グッズとそのカテゴリー・イベントを返し、削除できるかを管理者だけtrueにすることを検証する。
func TestGetAdminGoodsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminGoodsUsecase(newGoodsRepos(db, tx))
	eventID := testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).WithArchived("景品から外れたため").Build()

	for role, wantCanDelete := range map[model.UserRole]bool{model.UserRoleEditor: false, model.UserRoleAdmin: true} {
		output, err := uc.Execute(t.Context(), usecase.GetAdminGoodsInput{User: newUserWithRole(t, tx, role), GoodsID: goodsID})
		if err != nil {
			t.Fatalf("%s: Execute()のエラー = %v", role, err)
		}
		if output.Goods.ID != goodsID || output.EventCategory.ID != categoryID || output.Event.ID != eventID || output.CanDelete != wantCanDelete {
			t.Errorf("%s: 結果 = (%s, %s, %s, CanDelete %v)、(%s, %s, %s, CanDelete %v) を期待", role,
				output.Goods.ID, output.EventCategory.ID, output.Event.ID, output.CanDelete, goodsID, categoryID, eventID, wantCanDelete)
		}
	}
}

// TestGetAdminGoodsUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、削除したグッズ・
// 削除したカテゴリーのグッズ・削除したイベントのグッズ・無いグッズには AppErrCodeResourceNotFound を返すことを検証する。
func TestGetAdminGoodsUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminGoodsUsecase(newGoodsRepos(db, tx))
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	categoryID := newEventCategoryID(t, tx)

	_, err := uc.Execute(t.Context(), usecase.GetAdminGoodsInput{User: newUserWithRole(t, tx, model.UserRoleUser), GoodsID: testutil.NewGoodsBuilder(t, tx, categoryID).Build()})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	deletedEventCategoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).WithDeleted().Build()).Build()
	for name, id := range map[string]model.GoodsID{
		"削除したグッズ":       testutil.NewGoodsBuilder(t, tx, categoryID).WithDeleted().Build(),
		"削除したカテゴリーのグッズ": testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).WithDeleted().Build()).Build(),
		"削除したイベントのグッズ":  testutil.NewGoodsBuilder(t, tx, deletedEventCategoryID).Build(),
		"無いグッズ":         model.GoodsID(uuid.New()),
	} {
		_, err := uc.Execute(t.Context(), usecase.GetAdminGoodsInput{User: editor, GoodsID: id})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFound を期待", name, err)
		}
	}
}
