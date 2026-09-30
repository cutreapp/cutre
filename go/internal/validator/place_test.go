package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestPlaceUpdateValidator_Validate は、駅のIDを同じ駅を1つにまとめて読み、「ほかに出られるところ」の前後の空白を除くことと、
// 駅を1つも選ばなくても受け付けることを検証する。
func TestPlaceUpdateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	first, second := uuid.New(), uuid.New()

	output, err := validator.NewPlaceUpdateValidator().Validate(ctx, validator.PlaceUpdateValidatorInput{
		StationIDs: []string{first.String(), second.String(), first.String()},
		PlaceNote:  " 平日の夜なら ",
	})
	if err != nil || len(output.StationIDs) != 2 || output.StationIDs[0] != model.StationID(first) || output.StationIDs[1] != model.StationID(second) || output.PlaceNote != "平日の夜なら" {
		t.Errorf("Validate() = (%+v, %v)、2駅と前後の空白を除いたひとことを期待", output, err)
	}

	output, err = validator.NewPlaceUpdateValidator().Validate(ctx, validator.PlaceUpdateValidatorInput{})
	if err != nil || len(output.StationIDs) != 0 || output.PlaceNote != "" {
		t.Errorf("空のValidate() = (%+v, %v)、駅もひとことも無い結果を期待", output, err)
	}
}

// TestPlaceUpdateValidator_Validate_Invalid は、駅のIDとして読めない値をフォーム全体のエラーに、
// 長すぎる「ほかに出られるところ」をその欄のエラーにすることを検証する。
func TestPlaceUpdateValidator_Validate_Invalid(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	_, err := validator.NewPlaceUpdateValidator().Validate(ctx, validator.PlaceUpdateValidatorInput{
		StationIDs: []string{"not-a-uuid", "also-not-a-uuid"},
		PlaceNote:  strings.Repeat("あ", 201),
	})
	ve := model.AsValidationError(err)
	if ve == nil || len(ve.Global) != 1 || !ve.HasFieldError("place_note") {
		t.Errorf("Validate()のエラー = %v、フォーム全体のエラー1つと place_note のエラーを期待", err)
	}

	if _, err := validator.NewPlaceUpdateValidator().Validate(ctx, validator.PlaceUpdateValidatorInput{PlaceNote: strings.Repeat("あ", 200)}); err != nil {
		t.Errorf("200文字のValidate()のエラー = %v、受け付けることを期待", err)
	}
}

// TestPlaceStationUpdateValidator_Validate は、公開中の駅と、既に選んでいるアーカイブした駅を選べることと、
// 選んでいないアーカイブした駅・削除した駅・無い駅を選べないことを検証する。
func TestPlaceStationUpdateValidator_Validate(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewPlaceStationUpdateValidator(repository.NewStationRepository(db)).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	published := testutil.NewStationBuilder(t, tx).Build()
	archivedSelected := testutil.NewStationBuilder(t, tx).WithArchived("閉業").Build()
	testutil.NewUserStationBuilder(t, tx, userID, archivedSelected).Build()

	if err := v.Validate(ctx, userID, []model.StationID{published, archivedSelected}); err != nil {
		t.Errorf("選べる駅のValidate()のエラー = %v", err)
	}
	if err := v.Validate(ctx, userID, nil); err != nil {
		t.Errorf("空のValidate()のエラー = %v", err)
	}

	for name, unavailable := range map[string]model.StationID{
		"選んでいないアーカイブした駅": testutil.NewStationBuilder(t, tx).WithArchived("閉業").Build(),
		"削除した駅": testutil.NewStationBuilder(t, tx).WithDeleted().Build(),
		"無い駅":   model.StationID(uuid.New()),
	} {
		err := v.Validate(ctx, userID, []model.StationID{published, unavailable})
		if ve := model.AsValidationError(err); ve == nil || len(ve.Global) != 1 {
			t.Errorf("%sのValidate()のエラー = %v、フォーム全体の ValidationError を期待", name, err)
		}
	}
}
