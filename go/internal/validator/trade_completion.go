package validator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeCompletionValidator は「交換できた」のフォームを検証する。
type TradeCompletionValidator struct{}

// NewTradeCompletionValidator は TradeCompletionValidator を生成する。
func NewTradeCompletionValidator() *TradeCompletionValidator {
	return &TradeCompletionValidator{}
}

// TradeCompletionValidatorInput は TradeCompletionValidator.Validate の入力。ひとことはフォームの値をそのまま受け取る。
type TradeCompletionValidatorInput struct {
	Note string
}

// TradeCompletionValidateOutput は TradeCompletionValidator.Validate の結果。
type TradeCompletionValidateOutput struct {
	// Note は前後の空白と改行を除いたひとこと。入れていなければ空。
	Note string
}

// Validate は「交換できた」のひとことを検証する。
// ひとことは任意で、メッセージとして相手に届くため、メッセージの本文と同じ文字数までに限る。
// 入力の誤りは *model.ValidationError で返す。
func (v *TradeCompletionValidator) Validate(ctx context.Context, input TradeCompletionValidatorInput) (*TradeCompletionValidateOutput, error) {
	ve := model.NewValidationError()

	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > tradeMessageBodyMaxLength {
		ve.AddField("note", i18n.T(ctx, "validation_too_long", map[string]any{"Max": tradeMessageBodyMaxLength}))
	}

	if ve.HasErrors() {
		return nil, ve
	}

	return &TradeCompletionValidateOutput{Note: note}, nil
}
