package validator_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestTwoFactorAuthCreateValidator_Validate は、正規化したコードのうち半角数字6桁だけを受け付け、
// それ以外をコードの欄のエラーにすることを検証する。
func TestTwoFactorAuthCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewTwoFactorAuthCreateValidator()

	tests := []struct {
		name    string
		code    string
		wantErr string
	}{
		{name: "半角数字6桁", code: "012345"},
		{name: "未入力", code: "", wantErr: "入力してください"},
		{name: "5桁", code: "12345", wantErr: "6桁の数字で入力してください"},
		{name: "7桁", code: "1234567", wantErr: "6桁の数字で入力してください"},
		{name: "数字以外を含む", code: "12a456", wantErr: "6桁の数字で入力してください"},
	}
	for _, tt := range tests {
		err := v.Validate(ctx, validator.TwoFactorAuthCreateValidatorInput{Code: tt.code})
		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("%s: Validate()のエラー = %v、nilを期待", tt.name, err)
			}
			continue
		}

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Errorf("%s: Validate()のエラー = %v、ValidationErrorを期待", tt.name, err)
			continue
		}
		if got := ve.GetFieldErrors("code"); len(got) != 1 || got[0] != tt.wantErr {
			t.Errorf("%s: codeのエラー = %v、期待値 = [%s]", tt.name, got, tt.wantErr)
		}
	}
}

// TestTwoFactorAuthDeleteValidator_Validate は、再認証の入力が空のときだけ入力の欄のエラーにし、
// 形式は問わないことを検証する。
func TestTwoFactorAuthDeleteValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewTwoFactorAuthDeleteValidator()

	for _, credential := range []string{"123456", "short", "correct horse battery"} {
		if err := v.Validate(ctx, validator.TwoFactorAuthDeleteValidatorInput{Credential: credential}); err != nil {
			t.Errorf("%q: Validate()のエラー = %v、nilを期待", credential, err)
		}
	}

	ve := model.AsValidationError(v.Validate(ctx, validator.TwoFactorAuthDeleteValidatorInput{Credential: ""}))
	if ve == nil {
		t.Fatal("未入力のValidate()のエラー = nil、ValidationErrorを期待")
	}
	if got := ve.GetFieldErrors("credential"); len(got) != 1 || got[0] != "入力してください" {
		t.Errorf("credentialのエラー = %v、期待値 = [入力してください]", got)
	}
}
