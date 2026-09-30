package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// StationCreateValidator は管理画面の駅の作成のフォームを検証する。
type StationCreateValidator struct{}

// NewStationCreateValidator は StationCreateValidator を生成する。
func NewStationCreateValidator() *StationCreateValidator {
	return &StationCreateValidator{}
}

// StationCreateValidatorInput は StationCreateValidator.Validate の入力。フォームの値をそのまま受け取る。
type StationCreateValidatorInput struct {
	PrefectureCode string
	Name           string
	Position       string
}

// Validate は駅の作成のフォームを検証し、保存する属性を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *StationCreateValidator) Validate(ctx context.Context, input StationCreateValidatorInput) (*repository.StationAttributes, error) {
	return validateStationAttributes(ctx, input.PrefectureCode, input.Name, input.Position)
}

// StationUpdateValidator は管理画面の駅の編集のフォームを検証する。
// 検証する項目は作成と同じで、版の競合はUseCaseが更新のときに確かめる。
type StationUpdateValidator struct{}

// NewStationUpdateValidator は StationUpdateValidator を生成する。
func NewStationUpdateValidator() *StationUpdateValidator {
	return &StationUpdateValidator{}
}

// StationUpdateValidatorInput は StationUpdateValidator.Validate の入力。フォームの値をそのまま受け取る。
type StationUpdateValidatorInput struct {
	PrefectureCode string
	Name           string
	Position       string
}

// Validate は駅の編集のフォームを検証し、保存する属性を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *StationUpdateValidator) Validate(ctx context.Context, input StationUpdateValidatorInput) (*repository.StationAttributes, error) {
	return validateStationAttributes(ctx, input.PrefectureCode, input.Name, input.Position)
}

// validateStationAttributes は駅の都道府県・名前・並び順を検証し、保存する属性に変換する。
// 都道府県は選択肢から選ぶため、空の値と、都道府県コードとして読めない値を同じ「選んでください」のエラーにする。
func validateStationAttributes(ctx context.Context, prefectureCode, name, position string) (*repository.StationAttributes, error) {
	ve := model.NewValidationError()

	code, ok := model.ParsePrefectureCode(prefectureCode)
	if !ok {
		ve.AddField("prefecture_code", i18n.T(ctx, "validation_select_required"))
	}
	validName, validPosition := validateMasterNameAndPosition(ctx, ve, name, position)
	if ve.HasErrors() {
		return nil, ve
	}

	return &repository.StationAttributes{PrefectureCode: code, Name: validName, Position: validPosition}, nil
}
