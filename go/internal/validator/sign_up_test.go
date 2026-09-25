package validator_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestSignUpCreateValidator_Validate は、形式の誤りだけをフォームのエラーにし、
// 登録済みのアドレスはエラーにせずユーザーを返すことを検証する。
func TestSignUpCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewSignUpCreateValidator(repository.NewUserRepository(db).WithTx(tx))

	registered := testutil.UniqueEmail("sign-up-registered")
	registeredID := testutil.NewUserBuilder(t, tx).WithEmail(registered).Build()
	withdrawn := testutil.UniqueEmail("sign-up-withdrawn")
	testutil.NewUserBuilder(t, tx).WithEmail(withdrawn).WithDeletedAt(time.Now()).Build()

	tests := []struct {
		name         string
		email        string
		wantFieldErr bool
		wantUserID   *model.UserID
	}{
		{name: "未登録のアドレス", email: testutil.UniqueEmail("sign-up-new")},
		{name: "登録済みのアドレスはエラーにせずユーザーを返す", email: registered, wantUserID: &registeredID},
		{name: "退会したユーザーのアドレスは未登録として扱う", email: withdrawn},
		{name: "未入力", email: "", wantFieldErr: true},
		{name: "アドレスとして読めない", email: "not-an-email", wantFieldErr: true},
		{name: "表示名付きの形", email: "Cutre <user@example.com>", wantFieldErr: true},
	}

	for _, tt := range tests {
		user, err := v.Validate(ctx, validator.SignUpCreateValidatorInput{Email: tt.email})

		if tt.wantFieldErr {
			if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("email") {
				t.Errorf("%s: エラー = %v、emailのフィールドエラーを期待", tt.name, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: Validate()のエラー = %v", tt.name, err)
		}
		switch {
		case tt.wantUserID == nil && user != nil:
			t.Errorf("%s: ユーザー = %v、期待値 = nil", tt.name, user)
		case tt.wantUserID != nil && (user == nil || user.ID != *tt.wantUserID):
			t.Errorf("%s: ユーザー = %v、期待値 = %v", tt.name, user, *tt.wantUserID)
		}
	}
}
