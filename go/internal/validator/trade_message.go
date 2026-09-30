package validator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeMessageCreateValidator は交換のメッセージの送信のフォームを検証する。
type TradeMessageCreateValidator struct{}

// NewTradeMessageCreateValidator は TradeMessageCreateValidator を生成する。
func NewTradeMessageCreateValidator() *TradeMessageCreateValidator {
	return &TradeMessageCreateValidator{}
}

// TradeMessageCreateValidatorInput は TradeMessageCreateValidator.Validate の入力。本文はフォームの値をそのまま受け取る。
type TradeMessageCreateValidatorInput struct {
	Body string
}

// TradeMessageCreateValidateOutput は TradeMessageCreateValidator.Validate の結果。
type TradeMessageCreateValidateOutput struct {
	// Body は前後の空白と改行を除いた本文。本文の中の改行はそのまま残す。
	Body string
}

// Validate は交換のメッセージの本文を検証する。
// 本文は必須で、tradeMessageBodyMaxLength 文字までに限る。入力の誤りは *model.ValidationError で返す。
func (v *TradeMessageCreateValidator) Validate(ctx context.Context, input TradeMessageCreateValidatorInput) (*TradeMessageCreateValidateOutput, error) {
	ve := model.NewValidationError()

	body := strings.TrimSpace(input.Body)
	switch {
	case body == "":
		ve.AddField("body", i18n.T(ctx, "validation_required"))
	case utf8.RuneCountInString(body) > tradeMessageBodyMaxLength:
		ve.AddField("body", i18n.T(ctx, "validation_too_long", map[string]any{"Max": tradeMessageBodyMaxLength}))
	}

	if ve.HasErrors() {
		return nil, ve
	}

	return &TradeMessageCreateValidateOutput{Body: body}, nil
}
