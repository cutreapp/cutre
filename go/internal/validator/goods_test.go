package validator_test

import (
	"context"
	"slices"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestGoodsCreateValidator_Validate は、名前と並び順を保存する属性に変換し、誤りをそれぞれの欄のエラーにすることを検証する。
// 規則の組み合わせはカテゴリーと同じため、カテゴリーのテストで確かめる。
func TestGoodsCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	attrs, err := validator.NewGoodsCreateValidator().Validate(ctx, validator.GoodsCreateValidatorInput{Name: " くまの子 ", Position: "3"})
	if err != nil || attrs.Name != "くまの子" || attrs.Position != 3 {
		t.Errorf("Validate() = (%+v, %v)、名前「くまの子」・並び順3を期待", attrs, err)
	}

	_, err = validator.NewGoodsUpdateValidator().Validate(ctx, validator.GoodsUpdateValidatorInput{Name: "", Position: "x"})
	ve := model.AsValidationError(err)
	if ve == nil || !ve.HasFieldError("name") || !ve.HasFieldError("position") {
		t.Errorf("Validate()のエラー = %v、name と position の ValidationError を期待", err)
	}
}

// TestGoodsArchiveCreateValidator_Validate は、理由を前後の空白を除いて受け付け、空の理由をエラーにすることを検証する。
func TestGoodsArchiveCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewGoodsArchiveCreateValidator()

	message, err := v.Validate(ctx, validator.GoodsArchiveCreateValidatorInput{ArchiveMessage: " 景品から外れたため "})
	if err != nil || message != "景品から外れたため" {
		t.Errorf("Validate() = (%q, %v)、(%q, nil) を期待", message, err, "景品から外れたため")
	}

	_, err = v.Validate(ctx, validator.GoodsArchiveCreateValidatorInput{})
	if got := model.AsValidationError(err).GetFieldErrors("archive_message"); !slices.Equal(got, []string{"入力してください"}) {
		t.Errorf("空の archive_message のエラー = %v、期待値 = [入力してください]", got)
	}
}
