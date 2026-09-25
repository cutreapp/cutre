package validator_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestSignInTwoFactorRecoveryCreateValidator_Validate は、正規化したコードのうち発行するコードの形だけを受け付け、
// それ以外をコードの欄のエラーにすることを検証する。
func TestSignInTwoFactorRecoveryCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewSignInTwoFactorRecoveryCreateValidator()

	const formatErr = "リカバリーコードは8文字の英数字で入力してください"
	tests := []struct {
		name    string
		code    string
		wantErr string
	}{
		{name: "正規化した8文字", code: "abcd2345"},
		{name: "未入力", code: "", wantErr: "入力してください"},
		{name: "7文字", code: "abcd234", wantErr: formatErr},
		{name: "9文字", code: "abcd23456", wantErr: formatErr},
		{name: "使わない文字 (0)", code: "abcd2340", wantErr: formatErr},
		{name: "使わない文字 (o)", code: "abcd234o", wantErr: formatErr},
		{name: "使わない文字 (1)", code: "abcd2341", wantErr: formatErr},
		{name: "使わない文字 (l)", code: "abcd234l", wantErr: formatErr},
		{name: "使わない文字 (i)", code: "abcd234i", wantErr: formatErr},
	}
	for _, tt := range tests {
		err := v.Validate(ctx, validator.SignInTwoFactorRecoveryCreateValidatorInput{Code: tt.code})
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
