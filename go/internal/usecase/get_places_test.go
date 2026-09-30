package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetPlacesUsecase_Execute は、ユーザーが交換場所に選んだ駅を、アーカイブしたものを含めて返すことを検証する。
func TestGetPlacesUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetPlacesUsecase(repository.NewStationRepository(db).WithTx(tx), repository.NewUserRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserStationBuilder(t, tx, userID, testutil.NewStationBuilder(t, tx).Build()).Build()
	testutil.NewUserStationBuilder(t, tx, userID, testutil.NewStationBuilder(t, tx).WithArchived("閉業").Build()).Build()

	output, err := uc.Execute(t.Context(), usecase.GetPlacesInput{UserID: userID})
	if err != nil || output == nil || len(output.Stations) != 2 || output.User == nil || output.User.ID != userID || output.User.PlaceLockVersion != 0 {
		t.Errorf("Execute() = (%+v, %v)、ユーザーの版0と2駅を期待", output, err)
	}
}

// TestGetPublishedStationsUsecase_Execute は、公開中の駅だけを返すことを検証する。
func TestGetPublishedStationsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetPublishedStationsUsecase(repository.NewStationRepository(db).WithTx(tx))
	publishedID := testutil.NewStationBuilder(t, tx).Build()
	archivedID := testutil.NewStationBuilder(t, tx).WithArchived("閉業").Build()

	output, err := uc.Execute(t.Context())
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found := map[string]bool{}
	for _, station := range output.Stations {
		found[station.ID.String()] = true
	}
	if !found[publishedID.String()] || found[archivedID.String()] {
		t.Errorf("Execute() = %v、公開中の駅だけを期待", output.Stations)
	}
}
