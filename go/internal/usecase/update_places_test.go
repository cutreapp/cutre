package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newUpdatePlacesUsecase は、自分でトランザクションを開く UpdatePlacesUsecase をテスト用のデータベースで組み立てる。
func newUpdatePlacesUsecase() *usecase.UpdatePlacesUsecase {
	db := testutil.GetTestDB()
	stationRepo := repository.NewStationRepository(db)

	return usecase.NewUpdatePlacesUsecase(
		db, validator.NewPlaceUpdateValidator(), validator.NewPlaceStationUpdateValidator(stationRepo),
		stationRepo, repository.NewUserStationRepository(db), repository.NewUserRepository(db),
	)
}

// TestUpdatePlacesUsecase_Execute は、選んだ駅と「ほかに出られるところ」で交換場所を置き換えることを検証する。
func TestUpdatePlacesUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newUpdatePlacesUsecase()
	user := newCommittedUserWithRole(t, model.UserRoleUser)
	kept := testutil.NewStationBuilder(t, db).WithPosition(1).Build()
	removed := testutil.NewStationBuilder(t, db).WithPosition(2).Build()
	added := testutil.NewStationBuilder(t, db).WithPosition(3).Build()
	testutil.NewUserStationBuilder(t, db, user.ID, kept).Build()
	testutil.NewUserStationBuilder(t, db, user.ID, removed).Build()

	err := uc.Execute(jaContext(), usecase.UpdatePlacesInput{UserID: user.ID, StationIDs: []string{added.String(), kept.String()}, PlaceNote: "平日の夜なら"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	stations, err := repository.NewStationRepository(db).ListByUserID(t.Context(), user.ID)
	if err != nil || len(stations) != 2 || stations[0].ID != kept || stations[1].ID != added {
		t.Errorf("交換場所 = (%v, %v)、残した駅と足した駅を期待", stations, err)
	}
	if found, _ := repository.NewUserRepository(db).FindByID(t.Context(), user.ID); found.PlaceNote != "平日の夜なら" || found.PlaceLockVersion != 1 {
		t.Errorf("ユーザー = %+v、保存したひとことと版1を期待", found)
	}
}

// TestUpdatePlacesUsecase_Execute_Unavailable は、選べない駅を含むときは何も保存せず、
// 選び直しを案内する *model.ValidationError を返すことを検証する。
func TestUpdatePlacesUsecase_Execute_Unavailable(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newUpdatePlacesUsecase()
	user := newCommittedUserWithRole(t, model.UserRoleUser)
	published := testutil.NewStationBuilder(t, db).Build()
	archived := testutil.NewStationBuilder(t, db).WithArchived("閉業").Build()

	err := uc.Execute(jaContext(), usecase.UpdatePlacesInput{UserID: user.ID, StationIDs: []string{published.String(), archived.String()}, PlaceNote: "平日の夜なら"})
	if ve := model.AsValidationError(err); ve == nil || len(ve.Global) != 1 {
		t.Fatalf("エラー = %v、フォーム全体の *model.ValidationError を期待", err)
	}

	if exists, _ := repository.NewUserStationRepository(db).ExistsByUserID(t.Context(), user.ID); exists {
		t.Error("選べない駅を含む交換場所を保存した")
	}
	if found, _ := repository.NewUserRepository(db).FindByID(t.Context(), user.ID); found.PlaceNote != "" {
		t.Errorf("ほかに出られるところ = %q、保存しないことを期待", found.PlaceNote)
	}
}
