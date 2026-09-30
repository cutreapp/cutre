package validator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeDeclineValidator は交換のお断りのフォームを検証する。
type TradeDeclineValidator struct{}

// NewTradeDeclineValidator は TradeDeclineValidator を生成する。
func NewTradeDeclineValidator() *TradeDeclineValidator {
	return &TradeDeclineValidator{}
}

// TradeDeclineValidatorInput は TradeDeclineValidator.Validate の入力。理由とひとことはフォームの値をそのまま受け取る。
type TradeDeclineValidatorInput struct {
	Reason string
	Note   string
}

// TradeDeclineValidateOutput は TradeDeclineValidator.Validate の結果。
type TradeDeclineValidateOutput struct {
	Reason model.TradeDeclineReason
	// Note は前後の空白と改行を除いたひとこと。入れていなければ空。
	Note string
}

// Validate はお断りの理由とひとことを検証する。
// 理由は選択肢から1つを必須とする。ひとことは任意で、メッセージとして相手に届くため、メッセージの本文と同じ文字数までに限る。
// 入力の誤りは *model.ValidationError で返す。
func (v *TradeDeclineValidator) Validate(ctx context.Context, input TradeDeclineValidatorInput) (*TradeDeclineValidateOutput, error) {
	ve := model.NewValidationError()

	reason, ok := model.ParseTradeDeclineReason(input.Reason)
	if !ok {
		ve.AddField("reason", i18n.T(ctx, "validation_select_required"))
	}

	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > tradeMessageBodyMaxLength {
		ve.AddField("note", i18n.T(ctx, "validation_too_long", map[string]any{"Max": tradeMessageBodyMaxLength}))
	}

	if ve.HasErrors() {
		return nil, ve
	}

	return &TradeDeclineValidateOutput{Reason: reason, Note: note}, nil
}
