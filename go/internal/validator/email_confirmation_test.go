package validator_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestEmailConfirmationCreateValidator_Validate は、半角数字6桁だけを受け付け、それ以外をコードの欄のエラーにすることを検証する。
func TestEmailConfirmationCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewEmailConfirmationCreateValidator()

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
		{name: "全角数字", code: "１２３４５６", wantErr: "6桁の数字で入力してください"},
		{name: "前後の空白", code: " 123456", wantErr: "6桁の数字で入力してください"},
	}
	for _, tt := range tests {
		err := v.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{Code: tt.code})
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
