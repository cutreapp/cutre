package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestUpdateStationUsecase_Execute は、編集者が駅を更新でき、古い版からの更新には AppErrCodeConflict を、
// 削除した駅には AppErrCodeResourceNotFound を返すことを検証する。
func TestUpdateStationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	stationRepo := newStationRepo(db, tx)
	uc := usecase.NewUpdateStationUsecase(validator.NewStationUpdateValidator(), stationRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	id := testutil.NewStationBuilder(t, tx).Build()

	if err := uc.Execute(t.Context(), usecase.UpdateStationInput{User: editor, StationID: id, PrefectureCode: "14", Name: "横浜", Position: "4"}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := stationRepo.FindByID(t.Context(), id)
	if found.PrefectureCode != 14 || found.Name != "横浜" || found.Position != 4 || found.LockVersion != 1 {
		t.Errorf("更新した駅 = %+v、神奈川県 (14) の「横浜」・並び順4・版1を期待", found)
	}

	err := uc.Execute(t.Context(), usecase.UpdateStationInput{User: editor, StationID: id, LockVersion: 0, PrefectureCode: "14", Name: "古い版", Position: "4"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)

	err = uc.Execute(t.Context(), usecase.UpdateStationInput{User: editor, StationID: testutil.NewStationBuilder(t, tx).WithDeleted().Build(), PrefectureCode: "14", Name: "横浜", Position: "4"})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)
}
