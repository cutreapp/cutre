package validator

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// itemQuantityMax はアイテムの数量に入れられる最大の値。
// 手元にある数・ほしい数として現実的な範囲に収め、打ち間違いの大きな値を受け付けない。
const itemQuantityMax = 99

// itemNoteMaxLength はアイテムのひとことの最大の文字数。状態などを短く伝える欄のため、短めにする。
const itemNoteMaxLength = 200

// ItemCreateValidator はリストに追加するフォームを検証する。
type ItemCreateValidator struct{}

// NewItemCreateValidator は ItemCreateValidator を生成する。
func NewItemCreateValidator() *ItemCreateValidator {
	return &ItemCreateValidator{}
}

// ItemCreateValidatorInput は ItemCreateValidator.Validate の入力。フォームの値をそのまま受け取る。
type ItemCreateValidatorInput struct {
	Kind     string
	Quantity string
	Note     string
}

// Validate はリストに追加するフォームを検証し、保存する属性を返す。
//
// リストは選択肢から選ぶため、空の値と選択肢に無い値を同じ「選んでください」のエラーにする。
// 数量は1から itemQuantityMax までの整数に限る。0点のアイテムはリストに入れる意味が無いため。
// ひとことは任意で、前後の空白を除いて保存する。入力の誤りは *model.ValidationError で返す。
func (v *ItemCreateValidator) Validate(ctx context.Context, input ItemCreateValidatorInput) (*repository.ItemAttributes, error) {
	ve := model.NewValidationError()

	kind, ok := model.ParseItemKind(input.Kind)
	if !ok {
		ve.AddField("kind", i18n.T(ctx, "validation_select_required"))
	}

	quantity := validateItemQuantity(ctx, ve, input.Quantity)
	note := validateItemNote(ctx, ve, input.Note)

	if ve.HasErrors() {
		return nil, ve
	}

	return &repository.ItemAttributes{Kind: kind, Quantity: quantity, Note: note}, nil
}

// ItemUpdateValidator はリストのアイテムの編集のフォームを検証する。
type ItemUpdateValidator struct{}

// NewItemUpdateValidator は ItemUpdateValidator を生成する。
func NewItemUpdateValidator() *ItemUpdateValidator {
	return &ItemUpdateValidator{}
}

// ItemUpdateValidatorInput は ItemUpdateValidator.Validate の入力。フォームの値をそのまま受け取る。
type ItemUpdateValidatorInput struct {
	Quantity string
	Note     string
}

// Validate はリストのアイテムの編集のフォームを検証し、保存する数量とひとことを返す。
//
// 数量とひとことは、リストに追加するときと同じ決まりで検証する。
// 数量を0にしてリストから外すことはさせず、外すときは別の操作 (リストから外す) にする。
// リストは入れ替えられないため受け取らない。入力の誤りは *model.ValidationError で返す。
func (v *ItemUpdateValidator) Validate(ctx context.Context, input ItemUpdateValidatorInput) (*repository.ItemUpdateAttributes, error) {
	ve := model.NewValidationError()

	quantity := validateItemQuantity(ctx, ve, input.Quantity)
	note := validateItemNote(ctx, ve, input.Note)

	if ve.HasErrors() {
		return nil, ve
	}

	return &repository.ItemUpdateAttributes{Quantity: quantity, Note: note}, nil
}

// validateItemQuantity はアイテムの数量の入力を検証し、読めた値を返す。
// 数量は1から itemQuantityMax までの整数に限る。誤りは ve に足し、0を返す。
func validateItemQuantity(ctx context.Context, ve *model.ValidationError, raw string) int32 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		ve.AddField("quantity", i18n.T(ctx, "validation_required"))
		return 0
	}

	parsed, err := strconv.ParseInt(trimmed, 10, 32)
	if err != nil || parsed < 1 || parsed > itemQuantityMax {
		ve.AddField("quantity", i18n.T(ctx, "validation_quantity_out_of_range", map[string]any{"Max": itemQuantityMax}))
		return 0
	}

	return int32(parsed)
}

// validateItemNote はアイテムのひとことの入力を検証し、前後の空白を除いた値を返す。
// ひとことは任意で、itemNoteMaxLength 文字までに限る。誤りは ve に足す。
func validateItemNote(ctx context.Context, ve *model.ValidationError, raw string) string {
	note := strings.TrimSpace(raw)
	if utf8.RuneCountInString(note) > itemNoteMaxLength {
		ve.AddField("note", i18n.T(ctx, "validation_too_long", map[string]any{"Max": itemNoteMaxLength}))
	}

	return note
}
