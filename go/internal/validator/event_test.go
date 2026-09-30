package validator_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestEventCreateValidator_Validate は、名前と開始日があれば受け付けて保存する属性に変換し、
// 空・長すぎる名前、日付として読めない値、開始日より前の終了日をそれぞれの欄のエラーにすることを検証する。
// 編集のバリデーターも同じ規則のため、ここでまとめて確かめる。
func TestEventCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name         string
		input        validator.EventCreateValidatorInput
		wantName     []string
		wantStartsOn []string
		wantEndsOn   []string
	}{
		{name: "終了日あり", input: validator.EventCreateValidatorInput{Name: " 秋のくじ ", StartsOn: "2026-10-01", EndsOn: "2026-10-31"}},
		{name: "終了日が開始日と同じ", input: validator.EventCreateValidatorInput{Name: "秋のくじ", StartsOn: "2026-10-01", EndsOn: "2026-10-01"}},
		{name: "終了日なし", input: validator.EventCreateValidatorInput{Name: "秋のくじ", StartsOn: "2026-10-01"}},
		{name: "名前が空白だけ", input: validator.EventCreateValidatorInput{Name: "  ", StartsOn: "2026-10-01"}, wantName: []string{"入力してください"}},
		{name: "名前が長すぎる", input: validator.EventCreateValidatorInput{Name: strings.Repeat("あ", 101), StartsOn: "2026-10-01"}, wantName: []string{"100文字以内で入力してください"}},
		{name: "開始日が空", input: validator.EventCreateValidatorInput{Name: "秋のくじ"}, wantStartsOn: []string{"入力してください"}},
		{name: "開始日が読めない", input: validator.EventCreateValidatorInput{Name: "秋のくじ", StartsOn: "2026/10/01"}, wantStartsOn: []string{"日付を入力してください"}},
		{name: "終了日が読めない", input: validator.EventCreateValidatorInput{Name: "秋のくじ", StartsOn: "2026-10-01", EndsOn: "2026-02-30"}, wantEndsOn: []string{"日付を入力してください"}},
		{name: "終了日が開始日より前", input: validator.EventCreateValidatorInput{Name: "秋のくじ", StartsOn: "2026-10-01", EndsOn: "2026-09-30"}, wantEndsOn: []string{"開始日以降の日付を入力してください"}},
	}
	for _, tt := range tests {
		attrs, err := validator.NewEventCreateValidator().Validate(ctx, tt.input)
		if tt.wantName == nil && tt.wantStartsOn == nil && tt.wantEndsOn == nil {
			if err != nil || attrs == nil {
				t.Errorf("%s: Validate() = (%v, %v)、属性を期待", tt.name, attrs, err)
			}
			continue
		}

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Errorf("%s: Validate()のエラー = %v、ValidationErrorを期待", tt.name, err)
			continue
		}
		for field, want := range map[string][]string{"name": tt.wantName, "starts_on": tt.wantStartsOn, "ends_on": tt.wantEndsOn} {
			if got := ve.GetFieldErrors(field); !slices.Equal(got, want) {
				t.Errorf("%s: %s のエラー = %v、期待値 = %v", tt.name, field, got, want)
			}
		}
	}
}

// TestEventCreateValidator_Validate_Attributes は、名前の前後の空白を除き、日付を暦日 (UTCの0時) に変換し、
// 空の終了日を「終わりが決まっていない」(nil) にすることを検証する。
func TestEventCreateValidator_Validate_Attributes(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	attrs, err := validator.NewEventCreateValidator().Validate(ctx, validator.EventCreateValidatorInput{Name: " 秋のくじ ", StartsOn: "2026-10-01", EndsOn: "2026-10-31"})
	if err != nil {
		t.Fatalf("Validate()のエラー = %v", err)
	}
	if attrs.Name != "秋のくじ" {
		t.Errorf("Name = %q、期待値 = %q", attrs.Name, "秋のくじ")
	}
	if want := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC); !attrs.StartsOn.Equal(want) {
		t.Errorf("StartsOn = %v、期待値 = %v", attrs.StartsOn, want)
	}
	if want := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC); attrs.EndsOn == nil || !attrs.EndsOn.Equal(want) {
		t.Errorf("EndsOn = %v、期待値 = %v", attrs.EndsOn, want)
	}

	attrs, err = validator.NewEventUpdateValidator().Validate(ctx, validator.EventUpdateValidatorInput{Name: "秋のくじ", StartsOn: "2026-10-01"})
	if err != nil || attrs.EndsOn != nil {
		t.Errorf("終了日が空の Validate() = (%+v, %v)、EndsOn がnilの属性を期待", attrs, err)
	}
}

// TestEventArchiveCreateValidator_Validate は、理由を前後の空白を除いて受け付け、
// 空と長すぎる理由をエラーにすることを検証する。
func TestEventArchiveCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewEventArchiveCreateValidator()

	message, err := v.Validate(ctx, validator.EventArchiveCreateValidatorInput{ArchiveMessage: " 開催が終わったため "})
	if err != nil || message != "開催が終わったため" {
		t.Errorf("Validate() = (%q, %v)、(%q, nil) を期待", message, err, "開催が終わったため")
	}

	for name, tt := range map[string]struct {
		input string
		want  string
	}{
		"空白だけ": {input: " \n ", want: "入力してください"},
		"長すぎる": {input: strings.Repeat("あ", 501), want: "500文字以内で入力してください"},
	} {
		_, err := v.Validate(ctx, validator.EventArchiveCreateValidatorInput{ArchiveMessage: tt.input})
		if got := model.AsValidationError(err).GetFieldErrors("archive_message"); !slices.Equal(got, []string{tt.want}) {
			t.Errorf("%s: archive_message のエラー = %v、期待値 = [%s]", name, got, tt.want)
		}
	}
}
