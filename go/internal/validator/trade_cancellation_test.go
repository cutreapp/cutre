package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestTradeCancellationValidator_Validate は、選択肢の理由と、前後の空白と改行を除いた1000文字までのひとことを受け付けることを検証する。
func TestTradeCancellationValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewTradeCancellationValidator()

	output, err := v.Validate(ctx, validator.TradeCancellationValidatorInput{Reason: "decided_elsewhere", Note: "\n ごめんなさい、\n先に別の方と決まってしまいました。 \n"})
	if err != nil || output.Reason != model.TradeCancellationReasonDecidedElsewhere || output.Note != "ごめんなさい、\n先に別の方と決まってしまいました。" {
		t.Errorf("Validate() = (%+v, %v)、ほかの人と交換が決まった理由と、前後の空白と改行を除いたひとことを期待", output, err)
	}

	output, err = v.Validate(ctx, validator.TradeCancellationValidatorInput{Reason: "other", Note: strings.Repeat("あ", 1000)})
	if err != nil || output.Reason != model.TradeCancellationReasonOther {
		t.Errorf("ひとこと 1000文字: Validate() = (%+v, %v)、受け付けることを期待", output, err)
	}
}

// TestTradeCancellationValidator_Validate_Invalid は、選択肢に無い理由と空の理由を理由の欄の、
// 空 (空白と改行だけを含む) と1000文字を超えるひとことをひとことの欄のエラーにすることを検証する。
func TestTradeCancellationValidator_Validate_Invalid(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name  string
		input validator.TradeCancellationValidatorInput
		field string
		want  string
	}{
		{name: "理由が空", input: validator.TradeCancellationValidatorInput{Note: "ごめんなさい"}, field: "reason", want: "選んでください"},
		{name: "交換できなかったの理由", input: validator.TradeCancellationValidatorInput{Reason: "no_show", Note: "ごめんなさい"}, field: "reason", want: "選んでください"},
		{name: "ひとことが空", input: validator.TradeCancellationValidatorInput{Reason: "other"}, field: "note", want: "入力してください"},
		{name: "空白と改行だけのひとこと", input: validator.TradeCancellationValidatorInput{Reason: "other", Note: " \n "}, field: "note", want: "入力してください"},
		{name: "1000文字を超えるひとこと", input: validator.TradeCancellationValidatorInput{Reason: "other", Note: strings.Repeat("あ", 1001)}, field: "note", want: "1000文字以内で入力してください"},
	}
	for _, tt := range tests {
		_, err := validator.NewTradeCancellationValidator().Validate(ctx, tt.input)
		ve := model.AsValidationError(err)
		if ve == nil || !ve.HasFieldError(tt.field) || ve.Fields[tt.field][0] != tt.want {
			t.Errorf("%s: Validate()のエラー = %v、%s の「%s」を期待", tt.name, err, tt.field, tt.want)
		}
	}
}
