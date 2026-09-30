package validator_test

import (
	"context"
	"slices"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestStationCreateValidator_Validate は、都道府県・名前・並び順を保存する属性に変換し、誤りをそれぞれの欄のエラーにすることを検証する。
// 名前と並び順の規則はカテゴリーと同じため、カテゴリーのテストで確かめる。
func TestStationCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	attrs, err := validator.NewStationCreateValidator().Validate(ctx, validator.StationCreateValidatorInput{PrefectureCode: "13", Name: " 新宿 ", Position: "3"})
	if err != nil || attrs.PrefectureCode != 13 || attrs.Name != "新宿" || attrs.Position != 3 {
		t.Errorf("Validate() = (%+v, %v)、都道府県13・名前「新宿」・並び順3を期待", attrs, err)
	}

	_, err = validator.NewStationUpdateValidator().Validate(ctx, validator.StationUpdateValidatorInput{PrefectureCode: "13", Name: "", Position: "x"})
	ve := model.AsValidationError(err)
	if ve == nil || !ve.HasFieldError("name") || !ve.HasFieldError("position") || ve.HasFieldError("prefecture_code") {
		t.Errorf("Validate()のエラー = %v、name と position だけの ValidationError を期待", err)
	}
}

// TestStationCreateValidator_Validate_PrefectureCode は、空の都道府県と、都道府県コードとして読めない値を
// 「選んでください」のエラーにすることを検証する。
func TestStationCreateValidator_Validate_PrefectureCode(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	for _, code := range []string{"", "0", "48", "x", "1.5"} {
		_, err := validator.NewStationCreateValidator().Validate(ctx, validator.StationCreateValidatorInput{PrefectureCode: code, Name: "新宿", Position: "1"})
		if got := model.AsValidationError(err).GetFieldErrors("prefecture_code"); !slices.Equal(got, []string{"選んでください"}) {
			t.Errorf("prefecture_code %q のエラー = %v、期待値 = [選んでください]", code, got)
		}
	}
}

// TestStationArchiveCreateValidator_Validate は、理由を前後の空白を除いて受け付け、空の理由をエラーにすることを検証する。
func TestStationArchiveCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewStationArchiveCreateValidator()

	message, err := v.Validate(ctx, validator.StationArchiveCreateValidatorInput{ArchiveMessage: " 閉業したため "})
	if err != nil || message != "閉業したため" {
		t.Errorf("Validate() = (%q, %v)、(%q, nil) を期待", message, err, "閉業したため")
	}

	_, err = v.Validate(ctx, validator.StationArchiveCreateValidatorInput{})
	if got := model.AsValidationError(err).GetFieldErrors("archive_message"); !slices.Equal(got, []string{"入力してください"}) {
		t.Errorf("空の archive_message のエラー = %v、期待値 = [入力してください]", got)
	}
}
