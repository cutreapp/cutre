package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestUnarchiveStationUsecase_Execute は、編集者がアーカイブした駅を公開に戻せ、
// 公開中の駅を戻そうとすると AppErrCodeConflict を返すことを検証する。
func TestUnarchiveStationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	stationRepo := newStationRepo(db, tx)
	uc := usecase.NewUnarchiveStationUsecase(stationRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	id := testutil.NewStationBuilder(t, tx).WithArchived("閉業したため").Build()

	if err := uc.Execute(t.Context(), usecase.UnarchiveStationInput{User: editor, StationID: id}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := stationRepo.FindByID(t.Context(), id)
	if found.Status != model.MasterStatusPublished || found.ArchiveMessage != nil {
		t.Errorf("元に戻した駅 = %+v、公開中・理由なしを期待", found)
	}

	err := uc.Execute(t.Context(), usecase.UnarchiveStationInput{User: editor, StationID: id, LockVersion: 1})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
}
