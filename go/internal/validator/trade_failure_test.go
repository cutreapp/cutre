package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestTradeFailureValidator_Validate は、選択肢の理由を受け付け、ひとことの前後の空白と改行を除くことと、
// 1000文字までのひとことと、空のひとことを受け付けることを検証する。
func TestTradeFailureValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewTradeFailureValidator()

	output, err := v.Validate(ctx, validator.TradeFailureValidatorInput{Reason: "no_show", Note: "\n 30分待ちましたが、\n会えませんでした。 \n"})
	if err != nil || output.Reason != model.TradeFailureReasonNoShow || output.Note != "30分待ちましたが、\n会えませんでした。" {
		t.Errorf("Validate() = (%+v, %v)、当日会えなかった理由と、前後の空白と改行を除いたひとことを期待", output, err)
	}

	for _, note := range []string{"", " \n ", strings.Repeat("あ", 1000)} {
		output, err := v.Validate(ctx, validator.TradeFailureValidatorInput{Reason: "other", Note: note})
		if err != nil || output.Reason != model.TradeFailureReasonOther {
			t.Errorf("ひとこと %d文字: Validate() = (%+v, %v)、受け付けることを期待", len([]rune(note)), output, err)
		}
	}
}

// TestTradeFailureValidator_Validate_Invalid は、選択肢に無い理由と空の理由を理由の欄の、
// 1000文字を超えるひとことをひとことの欄のエラーにすることを検証する。
func TestTradeFailureValidator_Validate_Invalid(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name  string
		input validator.TradeFailureValidatorInput
		field string
		want  string
	}{
		{name: "理由が空", input: validator.TradeFailureValidatorInput{}, field: "reason", want: "選んでください"},
		{name: "お断りの理由", input: validator.TradeFailureValidatorInput{Reason: "place_mismatch"}, field: "reason", want: "選んでください"},
		{name: "1000文字を超えるひとこと", input: validator.TradeFailureValidatorInput{Reason: "other", Note: strings.Repeat("あ", 1001)}, field: "note", want: "1000文字以内で入力してください"},
	}
	for _, tt := range tests {
		_, err := validator.NewTradeFailureValidator().Validate(ctx, tt.input)
		ve := model.AsValidationError(err)
		if ve == nil || !ve.HasFieldError(tt.field) || ve.Fields[tt.field][0] != tt.want {
			t.Errorf("%s: Validate()のエラー = %v、%s の「%s」を期待", tt.name, err, tt.field, tt.want)
		}
	}
}
