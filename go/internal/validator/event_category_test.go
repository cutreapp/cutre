package validator_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestEventCategoryCreateValidator_Validate は、名前と並び順があれば受け付けて保存する属性に変換し、
// 空・長すぎる名前と、空・整数でない・範囲の外の並び順をそれぞれの欄のエラーにすることを検証する。
// 編集のバリデーターとグッズのバリデーターも同じ規則のため、規則の組み合わせはここでまとめて確かめる。
func TestEventCategoryCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name         string
		input        validator.EventCategoryCreateValidatorInput
		wantName     []string
		wantPosition []string
	}{
		{name: "並び順が0", input: validator.EventCategoryCreateValidatorInput{Name: "A賞", Position: "0"}},
		{name: "並び順が上限", input: validator.EventCategoryCreateValidatorInput{Name: "A賞", Position: "9999"}},
		{name: "名前が空白だけ", input: validator.EventCategoryCreateValidatorInput{Name: "  ", Position: "1"}, wantName: []string{"入力してください"}},
		{name: "名前が長すぎる", input: validator.EventCategoryCreateValidatorInput{Name: strings.Repeat("あ", 101), Position: "1"}, wantName: []string{"100文字以内で入力してください"}},
		{name: "並び順が空", input: validator.EventCategoryCreateValidatorInput{Name: "A賞", Position: " "}, wantPosition: []string{"入力してください"}},
		{name: "並び順が整数でない", input: validator.EventCategoryCreateValidatorInput{Name: "A賞", Position: "1.5"}, wantPosition: []string{"0から9999までの整数を入力してください"}},
		{name: "並び順が負", input: validator.EventCategoryCreateValidatorInput{Name: "A賞", Position: "-1"}, wantPosition: []string{"0から9999までの整数を入力してください"}},
		{name: "並び順が上限を超える", input: validator.EventCategoryCreateValidatorInput{Name: "A賞", Position: "10000"}, wantPosition: []string{"0から9999までの整数を入力してください"}},
		{name: "両方の誤り", input: validator.EventCategoryCreateValidatorInput{}, wantName: []string{"入力してください"}, wantPosition: []string{"入力してください"}},
	}
	for _, tt := range tests {
		attrs, err := validator.NewEventCategoryCreateValidator().Validate(ctx, tt.input)
		if tt.wantName == nil && tt.wantPosition == nil {
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
		for field, want := range map[string][]string{"name": tt.wantName, "position": tt.wantPosition} {
			if got := ve.GetFieldErrors(field); !slices.Equal(got, want) {
				t.Errorf("%s: %s のエラー = %v、期待値 = %v", tt.name, field, got, want)
			}
		}
	}
}

// TestEventCategoryUpdateValidator_Validate は、名前の前後の空白を除き、並び順を数値に変換することを検証する。
func TestEventCategoryUpdateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	attrs, err := validator.NewEventCategoryUpdateValidator().Validate(ctx, validator.EventCategoryUpdateValidatorInput{Name: " B賞 ラバーマスコット ", Position: " 20 "})
	if err != nil {
		t.Fatalf("Validate()のエラー = %v", err)
	}
	if attrs.Name != "B賞 ラバーマスコット" || attrs.Position != 20 {
		t.Errorf("属性 = %+v、名前「B賞 ラバーマスコット」・並び順20を期待", attrs)
	}
}

// TestEventCategoryArchiveCreateValidator_Validate は、理由を前後の空白を除いて受け付け、空の理由をエラーにすることを検証する。
// 理由の長さの上限はイベントと同じ規則のため、イベントのテストで確かめる。
func TestEventCategoryArchiveCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewEventCategoryArchiveCreateValidator()

	message, err := v.Validate(ctx, validator.EventCategoryArchiveCreateValidatorInput{ArchiveMessage: " 景品が変わったため "})
	if err != nil || message != "景品が変わったため" {
		t.Errorf("Validate() = (%q, %v)、(%q, nil) を期待", message, err, "景品が変わったため")
	}

	_, err = v.Validate(ctx, validator.EventCategoryArchiveCreateValidatorInput{ArchiveMessage: " "})
	if got := model.AsValidationError(err).GetFieldErrors("archive_message"); !slices.Equal(got, []string{"入力してください"}) {
		t.Errorf("空白だけの archive_message のエラー = %v、期待値 = [入力してください]", got)
	}
}
