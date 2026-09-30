package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestTradeMessageCreateValidator_Validate は、本文の前後の空白と改行を除き、本文の中の改行を残すことと、
// 1000文字までの本文を受け付けることを検証する。
func TestTradeMessageCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	output, err := validator.NewTradeMessageCreateValidator().Validate(ctx, validator.TradeMessageCreateValidatorInput{Body: "\n 土曜の13時に\n新宿駅はどうでしょう？ \n"})
	if err != nil || output.Body != "土曜の13時に\n新宿駅はどうでしょう？" {
		t.Errorf("Validate() = (%+v, %v)、前後の空白と改行を除いた本文を期待", output, err)
	}

	if _, err := validator.NewTradeMessageCreateValidator().Validate(ctx, validator.TradeMessageCreateValidatorInput{Body: strings.Repeat("あ", 1000)}); err != nil {
		t.Errorf("1000文字のValidate()のエラー = %v、受け付けることを期待", err)
	}
	if _, err := validator.NewTradeMessageCreateValidator().Validate(ctx, validator.TradeMessageCreateValidatorInput{Body: strings.Repeat("😀", 1000)}); err != nil {
		t.Errorf("絵文字1000文字のValidate()のエラー = %v、受け付けることを期待", err)
	}
}

// TestTradeMessageCreateValidator_Validate_Invalid は、空白と改行だけの本文と、1000文字を超える本文を、本文の欄のエラーにすることを検証する。
func TestTradeMessageCreateValidator_Validate_Invalid(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "空白と改行だけ", body: " \n ", want: "入力してください"},
		{name: "1000文字を超える", body: strings.Repeat("あ", 1001), want: "1000文字以内で入力してください"},
		{name: "絵文字で1000文字を超える", body: strings.Repeat("😀", 1001), want: "1000文字以内で入力してください"},
	}
	for _, tt := range tests {
		_, err := validator.NewTradeMessageCreateValidator().Validate(ctx, validator.TradeMessageCreateValidatorInput{Body: tt.body})
		ve := model.AsValidationError(err)
		if ve == nil || !ve.HasFieldError("body") || ve.Fields["body"][0] != tt.want {
			t.Errorf("%s: Validate()のエラー = %v、body の「%s」を期待", tt.name, err, tt.want)
		}
	}
}
