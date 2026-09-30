package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestTradeDeclineValidator_Validate は、選択肢の理由を受け付け、ひとことの前後の空白と改行を除くことと、
// 1000文字までのひとことと、空のひとことを受け付けることを検証する。
func TestTradeDeclineValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewTradeDeclineValidator()

	output, err := v.Validate(ctx, validator.TradeDeclineValidatorInput{Reason: "place_mismatch", Note: "\n 申し込みありがとうございます。\nまたの機会に。 \n"})
	if err != nil || output.Reason != model.TradeDeclineReasonPlaceMismatch || output.Note != "申し込みありがとうございます。\nまたの機会に。" {
		t.Errorf("Validate() = (%+v, %v)、交換場所が合わない理由と、前後の空白と改行を除いたひとことを期待", output, err)
	}

	for _, note := range []string{"", " \n ", strings.Repeat("あ", 1000)} {
		output, err := v.Validate(ctx, validator.TradeDeclineValidatorInput{Reason: "other", Note: note})
		if err != nil || output.Reason != model.TradeDeclineReasonOther {
			t.Errorf("ひとこと %d文字: Validate() = (%+v, %v)、受け付けることを期待", len([]rune(note)), output, err)
		}
	}
}

// TestTradeDeclineValidator_Validate_Invalid は、選択肢に無い理由と空の理由を理由の欄の、
// 1000文字を超えるひとことをひとことの欄のエラーにすることを検証する。
func TestTradeDeclineValidator_Validate_Invalid(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name  string
		input validator.TradeDeclineValidatorInput
		field string
		want  string
	}{
		{name: "理由が空", input: validator.TradeDeclineValidatorInput{}, field: "reason", want: "選んでください"},
		{name: "選択肢に無い理由", input: validator.TradeDeclineValidatorInput{Reason: "rude"}, field: "reason", want: "選んでください"},
		{name: "1000文字を超えるひとこと", input: validator.TradeDeclineValidatorInput{Reason: "other", Note: strings.Repeat("あ", 1001)}, field: "note", want: "1000文字以内で入力してください"},
	}
	for _, tt := range tests {
		_, err := validator.NewTradeDeclineValidator().Validate(ctx, tt.input)
		ve := model.AsValidationError(err)
		if ve == nil || !ve.HasFieldError(tt.field) || ve.Fields[tt.field][0] != tt.want {
			t.Errorf("%s: Validate()のエラー = %v、%s の「%s」を期待", tt.name, err, tt.field, tt.want)
		}
	}
}
