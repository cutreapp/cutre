package validator

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// eventNameMaxLength はイベントの名前の最大の文字数。
// 一覧の1行に収まり、公式の長い表記も入る長さにする。
const eventNameMaxLength = 100

// dateLayout は日付の入力欄 (<input type="date">) が送る値の形。
const dateLayout = "2006-01-02"

// EventCreateValidator は管理画面のイベントの作成のフォームを検証する。
type EventCreateValidator struct{}

// NewEventCreateValidator は EventCreateValidator を生成する。
func NewEventCreateValidator() *EventCreateValidator {
	return &EventCreateValidator{}
}

// EventCreateValidatorInput は EventCreateValidator.Validate の入力。フォームの値をそのまま受け取る。
type EventCreateValidatorInput struct {
	Name     string
	StartsOn string
	EndsOn   string
}

// Validate はイベントの作成のフォームを検証し、保存する属性を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *EventCreateValidator) Validate(ctx context.Context, input EventCreateValidatorInput) (*repository.EventAttributes, error) {
	return validateEventAttributes(ctx, input.Name, input.StartsOn, input.EndsOn)
}

// EventUpdateValidator は管理画面のイベントの編集のフォームを検証する。
// 検証する項目は作成と同じで、版の競合はUseCaseが更新のときに確かめる。
type EventUpdateValidator struct{}

// NewEventUpdateValidator は EventUpdateValidator を生成する。
func NewEventUpdateValidator() *EventUpdateValidator {
	return &EventUpdateValidator{}
}

// EventUpdateValidatorInput は EventUpdateValidator.Validate の入力。フォームの値をそのまま受け取る。
type EventUpdateValidatorInput struct {
	Name     string
	StartsOn string
	EndsOn   string
}

// Validate はイベントの編集のフォームを検証し、保存する属性を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *EventUpdateValidator) Validate(ctx context.Context, input EventUpdateValidatorInput) (*repository.EventAttributes, error) {
	return validateEventAttributes(ctx, input.Name, input.StartsOn, input.EndsOn)
}

// validateEventAttributes はイベントの名前と開催期間を検証し、保存する属性に変換する。
//
// 名前は前後の空白を除いて保存する。終わりの日は空なら「終わりが決まっていない」とし、
// 入力されたときは開始日より前にできない。
func validateEventAttributes(ctx context.Context, name, startsOn, endsOn string) (*repository.EventAttributes, error) {
	ve := model.NewValidationError()
	attrs := &repository.EventAttributes{Name: strings.TrimSpace(name)}

	switch {
	case attrs.Name == "":
		ve.AddField("name", i18n.T(ctx, "validation_required"))
	case utf8.RuneCountInString(attrs.Name) > eventNameMaxLength:
		ve.AddField("name", i18n.T(ctx, "validation_too_long", map[string]any{"Max": eventNameMaxLength}))
	}

	if startsOn == "" {
		ve.AddField("starts_on", i18n.T(ctx, "validation_required"))
	} else if parsed, err := time.Parse(dateLayout, startsOn); err != nil {
		ve.AddField("starts_on", i18n.T(ctx, "validation_date_invalid"))
	} else {
		attrs.StartsOn = parsed
	}

	if endsOn != "" {
		parsed, err := time.Parse(dateLayout, endsOn)
		switch {
		case err != nil:
			ve.AddField("ends_on", i18n.T(ctx, "validation_date_invalid"))
		case !attrs.StartsOn.IsZero() && parsed.Before(attrs.StartsOn):
			ve.AddField("ends_on", i18n.T(ctx, "validation_event_ends_on_before_starts_on"))
		default:
			attrs.EndsOn = &parsed
		}
	}

	if ve.HasErrors() {
		return nil, ve
	}

	return attrs, nil
}
