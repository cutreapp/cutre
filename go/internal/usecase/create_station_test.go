package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestCreateStationUsecase_Execute は、編集者が公開中の駅を作成でき、フォームの誤りには *model.ValidationError を、
// 一般のユーザーには AppErrCodeForbidden を返すことを検証する。
func TestCreateStationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	stationRepo := newStationRepo(db, tx)
	uc := usecase.NewCreateStationUsecase(validator.NewStationCreateValidator(), stationRepo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)

	output, err := uc.Execute(t.Context(), usecase.CreateStationInput{User: editor, PrefectureCode: "27", Name: "梅田", Position: "2"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := stationRepo.FindByID(t.Context(), output.Station.ID)
	if found == nil || found.PrefectureCode != 27 || found.Name != "梅田" || found.Position != 2 || found.Status != model.MasterStatusPublished {
		t.Errorf("作成した駅 = %+v、大阪府 (27) の公開中の「梅田」・並び順2を期待", found)
	}

	_, err = uc.Execute(jaContext(), usecase.CreateStationInput{User: editor, PrefectureCode: "", Name: "梅田", Position: "2"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("prefecture_code") {
		t.Errorf("都道府県が空のエラー = %v、prefecture_code の ValidationError を期待", err)
	}

	_, err = uc.Execute(t.Context(), usecase.CreateStationInput{User: newUserWithRole(t, tx, model.UserRoleUser), PrefectureCode: "27", Name: "梅田", Position: "2"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)
}
