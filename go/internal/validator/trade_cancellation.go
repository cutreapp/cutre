package validator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeCancellationValidator は交換をやめるフォームを検証する。
type TradeCancellationValidator struct{}

// NewTradeCancellationValidator は TradeCancellationValidator を生成する。
func NewTradeCancellationValidator() *TradeCancellationValidator {
	return &TradeCancellationValidator{}
}

// TradeCancellationValidatorInput は TradeCancellationValidator.Validate の入力。理由とひとことはフォームの値をそのまま受け取る。
type TradeCancellationValidatorInput struct {
	Reason string
	Note   string
}

// TradeCancellationValidateOutput は TradeCancellationValidator.Validate の結果。
type TradeCancellationValidateOutput struct {
	Reason model.TradeCancellationReason
	// Note は前後の空白と改行を除いたひとこと。
	Note string
}

// Validate は交換をやめる理由とひとことを検証する。
// 理由は選択肢から1つを必須とする。ひとことは、会う約束をしたあとに一方的に終えることになるため必須とし、
// メッセージとして相手に届くため、メッセージの本文と同じ文字数までに限る。空白と改行だけのひとことは空として扱う。
// 入力の誤りは *model.ValidationError で返す。
func (v *TradeCancellationValidator) Validate(ctx context.Context, input TradeCancellationValidatorInput) (*TradeCancellationValidateOutput, error) {
	ve := model.NewValidationError()

	reason, ok := model.ParseTradeCancellationReason(input.Reason)
	if !ok {
		ve.AddField("reason", i18n.T(ctx, "validation_select_required"))
	}

	note := strings.TrimSpace(input.Note)
	switch {
	case note == "":
		ve.AddField("note", i18n.T(ctx, "validation_required"))
	case utf8.RuneCountInString(note) > tradeMessageBodyMaxLength:
		ve.AddField("note", i18n.T(ctx, "validation_too_long", map[string]any{"Max": tradeMessageBodyMaxLength}))
	}

	if ve.HasErrors() {
		return nil, ve
	}

	return &TradeCancellationValidateOutput{Reason: reason, Note: note}, nil
}
