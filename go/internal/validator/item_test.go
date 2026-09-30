package validator_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestItemCreateValidator_Validate は、リスト・数量・ひとことを保存する属性に変換し、誤りをそれぞれの欄のエラーにすることを検証する。
func TestItemCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewItemCreateValidator()

	attrs, err := v.Validate(ctx, validator.ItemCreateValidatorInput{Kind: "want", Quantity: " 3 ", Note: " 色違いでも可 "})
	if err != nil || *attrs != (repository.ItemAttributes{Kind: model.ItemKindWant, Quantity: 3, Note: "色違いでも可"}) {
		t.Errorf("Validate() = (%+v, %v)、ほしい・3点・「色違いでも可」を期待", attrs, err)
	}

	attrs, err = v.Validate(ctx, validator.ItemCreateValidatorInput{Kind: "give", Quantity: "99"})
	if err != nil || attrs.Quantity != 99 || attrs.Note != "" {
		t.Errorf("Validate() = (%+v, %v)、上限の99点・ひとことなしを受け付けることを期待", attrs, err)
	}

	tests := []struct {
		name  string
		input validator.ItemCreateValidatorInput
		field string
		want  string
	}{
		{name: "リストが空", input: validator.ItemCreateValidatorInput{Quantity: "1"}, field: "kind", want: "選んでください"},
		{name: "選択肢に無いリスト", input: validator.ItemCreateValidatorInput{Kind: "trade", Quantity: "1"}, field: "kind", want: "選んでください"},
		{name: "数量が空", input: validator.ItemCreateValidatorInput{Kind: "give"}, field: "quantity", want: "入力してください"},
		{name: "数量が0", input: validator.ItemCreateValidatorInput{Kind: "give", Quantity: "0"}, field: "quantity", want: "1から99までの整数を入力してください"},
		{name: "数量が上限を超える", input: validator.ItemCreateValidatorInput{Kind: "give", Quantity: "100"}, field: "quantity", want: "1から99までの整数を入力してください"},
		{name: "数量が整数でない", input: validator.ItemCreateValidatorInput{Kind: "give", Quantity: "1.5"}, field: "quantity", want: "1から99までの整数を入力してください"},
		{name: "ひとことが長すぎる", input: validator.ItemCreateValidatorInput{Kind: "give", Quantity: "1", Note: strings.Repeat("あ", 201)}, field: "note", want: "200文字以内で入力してください"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := v.Validate(ctx, tt.input)
			if got := model.AsValidationError(err).GetFieldErrors(tt.field); !slices.Equal(got, []string{tt.want}) {
				t.Errorf("%s のエラー = %v、期待値 = [%s]", tt.field, got, tt.want)
			}
		})
	}
}

// TestItemUpdateValidator_Validate は、数量とひとことを保存する値に変換し、誤りをそれぞれの欄のエラーにすることを検証する。
func TestItemUpdateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewItemUpdateValidator()

	output, err := v.Validate(ctx, validator.ItemUpdateValidatorInput{Quantity: " 4 ", Note: " 未開封 "})
	if err != nil || *output != (repository.ItemUpdateAttributes{Quantity: 4, Note: "未開封"}) {
		t.Errorf("Validate() = (%+v, %v)、4点・「未開封」を期待", output, err)
	}

	tests := []struct {
		name  string
		input validator.ItemUpdateValidatorInput
		field string
		want  string
	}{
		{name: "数量が空", input: validator.ItemUpdateValidatorInput{}, field: "quantity", want: "入力してください"},
		{name: "数量が0", input: validator.ItemUpdateValidatorInput{Quantity: "0"}, field: "quantity", want: "1から99までの整数を入力してください"},
		{name: "数量が上限を超える", input: validator.ItemUpdateValidatorInput{Quantity: "100"}, field: "quantity", want: "1から99までの整数を入力してください"},
		{name: "ひとことが長すぎる", input: validator.ItemUpdateValidatorInput{Quantity: "1", Note: strings.Repeat("あ", 201)}, field: "note", want: "200文字以内で入力してください"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := v.Validate(ctx, tt.input)
			if got := model.AsValidationError(err).GetFieldErrors(tt.field); !slices.Equal(got, []string{tt.want}) {
				t.Errorf("%s のエラー = %v、期待値 = [%s]", tt.field, got, tt.want)
			}
		})
	}
}
