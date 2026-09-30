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

// newStationRepo はテストのトランザクションの中で動く、駅のUseCaseが使うリポジトリを作る。
func newStationRepo(db *sql.DB, tx *sql.Tx) *repository.StationRepository {
	return repository.NewStationRepository(db).WithTx(tx)
}

// TestGetAdminStationsUsecase_Execute は、編集者に削除していない駅を返し、一般のユーザーには AppErrCodeForbidden を返すことを検証する。
func TestGetAdminStationsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminStationsUsecase(newStationRepo(db, tx))
	archivedID := testutil.NewStationBuilder(t, tx).WithArchived("閉業したため").Build()
	deletedID := testutil.NewStationBuilder(t, tx).WithDeleted().Build()

	output, err := uc.Execute(t.Context(), usecase.GetAdminStationsInput{User: newUserWithRole(t, tx, model.UserRoleEditor)})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	var foundArchived bool
	for _, station := range output.Stations {
		if station.ID == deletedID {
			t.Error("削除した駅を返している")
		}
		foundArchived = foundArchived || station.ID == archivedID
	}
	if !foundArchived {
		t.Error("アーカイブした駅を返していない")
	}

	_, err = uc.Execute(t.Context(), usecase.GetAdminStationsInput{User: newUserWithRole(t, tx, model.UserRoleUser)})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)
}

// TestGetAdminStationUsecase_Execute は、駅を返して削除できるかを管理者だけtrueにし、
// 一般のユーザーには AppErrCodeForbidden を、削除した駅と無い駅には AppErrCodeResourceNotFound を返すことを検証する。
func TestGetAdminStationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminStationUsecase(newStationRepo(db, tx))
	id := testutil.NewStationBuilder(t, tx).WithArchived("閉業したため").Build()

	for role, wantCanDelete := range map[model.UserRole]bool{model.UserRoleEditor: false, model.UserRoleAdmin: true} {
		output, err := uc.Execute(t.Context(), usecase.GetAdminStationInput{User: newUserWithRole(t, tx, role), StationID: id})
		if err != nil {
			t.Fatalf("%s: Execute()のエラー = %v", role, err)
		}
		if output.Station.ID != id || output.CanDelete != wantCanDelete {
			t.Errorf("%s: 結果 = (%s, CanDelete %v)、(%s, CanDelete %v) を期待", role, output.Station.ID, output.CanDelete, id, wantCanDelete)
		}
	}

	_, err := uc.Execute(t.Context(), usecase.GetAdminStationInput{User: newUserWithRole(t, tx, model.UserRoleUser), StationID: id})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	for name, stationID := range map[string]model.StationID{
		"削除した駅": testutil.NewStationBuilder(t, tx).WithDeleted().Build(),
		"無い駅":   model.StationID(uuid.New()),
	} {
		_, err := uc.Execute(t.Context(), usecase.GetAdminStationInput{User: editor, StationID: stationID})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFound を期待", name, err)
		}
	}
}
