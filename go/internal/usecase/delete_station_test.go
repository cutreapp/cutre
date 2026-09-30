package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newDeleteStationUsecase は、自分でトランザクションを開く DeleteStationUsecase をテスト用のデータベースで組み立てる。
func newDeleteStationUsecase() (*usecase.DeleteStationUsecase, *repository.StationRepository) {
	db := testutil.GetTestDB()
	stationRepo := repository.NewStationRepository(db)

	return usecase.NewDeleteStationUsecase(db, validator.NewStationDeleteValidator(repository.NewUserStationRepository(db)), stationRepo), stationRepo
}

// TestDeleteStationUsecase_Execute は、管理者だけが駅を削除でき、編集者には AppErrCodeForbidden を、
// 古い版からの削除には AppErrCodeConflict を返すことを検証する。
func TestDeleteStationUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, stationRepo := newDeleteStationUsecase()
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	id := testutil.NewStationBuilder(t, testutil.GetTestDB()).Build()

	err := uc.Execute(t.Context(), usecase.DeleteStationInput{User: newCommittedUserWithRole(t, model.UserRoleEditor), StationID: id})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	err = uc.Execute(t.Context(), usecase.DeleteStationInput{User: admin, StationID: id, LockVersion: 1})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)

	if err := uc.Execute(t.Context(), usecase.DeleteStationInput{User: admin, StationID: id}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if found, _ := stationRepo.FindByID(t.Context(), id); !found.IsDeleted() {
		t.Errorf("状態 = %q、削除した状態を期待", found.Status)
	}
}

// TestDeleteStationUsecase_Execute_Referenced は、駅を交換場所に選んでいるユーザーがいれば削除せず、
// アーカイブを案内する *model.ValidationError を返すことを検証する。
func TestDeleteStationUsecase_Execute_Referenced(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc, stationRepo := newDeleteStationUsecase()
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	id := testutil.NewStationBuilder(t, db).WithArchived("閉店").Build()
	testutil.NewUserStationBuilder(t, db, admin.ID, id).Build()

	err := uc.Execute(jaContext(), usecase.DeleteStationInput{User: admin, StationID: id})
	if ve := model.AsValidationError(err); ve == nil || len(ve.Global) != 1 {
		t.Fatalf("エラー = %v、フォーム全体の *model.ValidationError を期待", err)
	}
	if found, _ := stationRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("交換場所に選ばれている駅を削除した")
	}
}
