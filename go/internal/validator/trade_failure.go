package validator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeFailureValidator は「交換できなかった」のフォームを検証する。
type TradeFailureValidator struct{}

// NewTradeFailureValidator は TradeFailureValidator を生成する。
func NewTradeFailureValidator() *TradeFailureValidator {
	return &TradeFailureValidator{}
}

// TradeFailureValidatorInput は TradeFailureValidator.Validate の入力。理由とひとことはフォームの値をそのまま受け取る。
type TradeFailureValidatorInput struct {
	Reason string
	Note   string
}

// TradeFailureValidateOutput は TradeFailureValidator.Validate の結果。
type TradeFailureValidateOutput struct {
	Reason model.TradeFailureReason
	// Note は前後の空白と改行を除いたひとこと。入れていなければ空。
	Note string
}

// Validate は「交換できなかった」の理由とひとことを検証する。
// 理由は選択肢から1つを必須とする。ひとことは任意で、メッセージとして相手に届くため、メッセージの本文と同じ文字数までに限る。
// 入力の誤りは *model.ValidationError で返す。
func (v *TradeFailureValidator) Validate(ctx context.Context, input TradeFailureValidatorInput) (*TradeFailureValidateOutput, error) {
	ve := model.NewValidationError()

	reason, ok := model.ParseTradeFailureReason(input.Reason)
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

	return &TradeFailureValidateOutput{Reason: reason, Note: note}, nil
}
