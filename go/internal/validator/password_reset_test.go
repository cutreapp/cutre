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

// TestPasswordResetCreateValidator_Validate は、形式の誤りだけをフォームのエラーにし、
// 登録済みのアドレスではユーザーを、未登録・退会済みのアドレスではnilを返すことを検証する。
func TestPasswordResetCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewPasswordResetCreateValidator(repository.NewUserRepository(db).WithTx(tx))

	registered := testutil.UniqueEmail("password-reset-registered")
	registeredID := testutil.NewUserBuilder(t, tx).WithEmail(registered).Build()
	withdrawn := testutil.UniqueEmail("password-reset-withdrawn")
	testutil.NewUserBuilder(t, tx).WithEmail(withdrawn).WithDeletedAt(time.Now()).Build()

	tests := []struct {
		name         string
		email        string
		wantFieldErr bool
		wantUserID   *model.UserID
	}{
		{name: "未登録のアドレスはエラーにせずnilを返す", email: testutil.UniqueEmail("password-reset-unknown")},
		{name: "登録済みのアドレス", email: registered, wantUserID: &registeredID},
		{name: "退会したユーザーのアドレスは未登録として扱う", email: withdrawn},
		{name: "未入力", email: "", wantFieldErr: true},
		{name: "アドレスとして読めない", email: "not-an-email", wantFieldErr: true},
	}

	for _, tt := range tests {
		user, err := v.Validate(ctx, validator.PasswordResetCreateValidatorInput{Email: tt.email})

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
