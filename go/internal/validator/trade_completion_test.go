package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestTradeCompletionValidator_Validate は、ひとことの前後の空白と改行を除くことと、
// 1000文字までのひとことと、空のひとことを受け付けることを検証する。
func TestTradeCompletionValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewTradeCompletionValidator()

	output, err := v.Validate(ctx, validator.TradeCompletionValidatorInput{Note: "\n ありがとうございました。\n大事にします。 \n"})
	if err != nil || output.Note != "ありがとうございました。\n大事にします。" {
		t.Errorf("Validate() = (%+v, %v)、前後の空白と改行を除いたひとことを期待", output, err)
	}

	for _, note := range []string{"", " \n ", strings.Repeat("あ", 1000)} {
		if _, err := v.Validate(ctx, validator.TradeCompletionValidatorInput{Note: note}); err != nil {
			t.Errorf("ひとこと %d文字: Validate()のエラー = %v、受け付けることを期待", len([]rune(note)), err)
		}
	}
}

// TestTradeCompletionValidator_Validate_Invalid は、1000文字を超えるひとことをひとことの欄のエラーにすることを検証する。
func TestTradeCompletionValidator_Validate_Invalid(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	_, err := validator.NewTradeCompletionValidator().Validate(ctx, validator.TradeCompletionValidatorInput{Note: strings.Repeat("あ", 1001)})
	ve := model.AsValidationError(err)
	if ve == nil || !ve.HasFieldError("note") || ve.Fields["note"][0] != "1000文字以内で入力してください" {
		t.Errorf("Validate()のエラー = %v、note の「1000文字以内で入力してください」を期待", err)
	}
}
