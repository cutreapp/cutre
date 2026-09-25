package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestPasswordUpdateValidator_Validate は、新しいパスワードにアカウントの作成と同じ規則を当てることを検証する。
func TestPasswordUpdateValidator_Validate(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewPasswordUpdateValidator()

	tests := []struct {
		name     string
		password string
		wantErr  string
	}{
		{name: "8文字のパスワード", password: "abcdefgh"},
		{name: "72バイトのパスワード", password: strings.Repeat("a", 72)},
		{name: "未入力", password: "", wantErr: "入力してください"},
		{name: "7文字のパスワード", password: "abcdefg", wantErr: "8文字以上で入力してください"},
		{name: "73バイトのパスワード", password: strings.Repeat("a", 73), wantErr: "長すぎます"},
	}

	for _, tt := range tests {
		err := v.Validate(ctx, validator.PasswordUpdateValidatorInput{Password: tt.password})

		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("%s: エラー = %v、nilを期待", tt.name, err)
			}
			continue
		}
		ve := model.AsValidationError(err)
		if ve == nil {
			t.Errorf("%s: エラー = %v、ValidationErrorを期待", tt.name, err)
			continue
		}
		assertMessage(t, tt.name, ve.GetFieldErrors("password"), tt.wantErr)
	}
}
