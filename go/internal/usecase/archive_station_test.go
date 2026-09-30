package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestArchiveStationUsecase_Execute は、編集者が理由を残して駅をアーカイブでき、理由が空のときは
// *model.ValidationError を、もう一度アーカイブしようとすると AppErrCodeConflict を返すことを検証する。
func TestArchiveStationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	stationRepo := newStationRepo(db, tx)
	uc := usecase.NewArchiveStationUsecase(validator.NewStationArchiveCreateValidator(), stationRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	id := testutil.NewStationBuilder(t, tx).Build()

	err := uc.Execute(jaContext(), usecase.ArchiveStationInput{User: editor, StationID: id})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("archive_message") {
		t.Errorf("理由が空のエラー = %v、archive_message の ValidationError を期待", err)
	}

	if err := uc.Execute(t.Context(), usecase.ArchiveStationInput{User: editor, StationID: id, ArchiveMessage: "閉業したため"}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := stationRepo.FindByID(t.Context(), id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "閉業したため" {
		t.Errorf("アーカイブした駅 = %+v、理由を残したアーカイブを期待", found)
	}

	// 状態だけで拒むことを確かめるため、今の版を送る。
	err = uc.Execute(t.Context(), usecase.ArchiveStationInput{User: editor, StationID: id, LockVersion: 1, ArchiveMessage: "もう一度"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
}
