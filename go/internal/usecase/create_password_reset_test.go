package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newCreatePasswordResetUsecase はテスト用のデータベースに直接書き込む CreatePasswordResetUsecase を組み立てる。
func newCreatePasswordResetUsecase(t *testing.T) *usecase.CreatePasswordResetUsecase {
	t.Helper()

	db := testutil.GetTestDB()
	jobs, err := dispatcher.NewDispatcher(db)
	if err != nil {
		t.Fatalf("NewDispatcher()のエラー = %v", err)
	}

	return usecase.NewCreatePasswordResetUsecase(validator.NewPasswordResetCreateValidator(repository.NewUserRepository(db)), jobs)
}

// passwordResetJobLocales は指定したユーザーへ投入されたパスワードリセットのジョブの言語を返す。
func passwordResetJobLocales(t *testing.T, userID model.UserID) []string {
	t.Helper()

	rows, err := testutil.GetTestDB().QueryContext(context.Background(),
		"SELECT args->>'locale' FROM river_job WHERE kind = 'send_password_reset' AND args->>'user_id' = $1", userID.String())
	if err != nil {
		t.Fatalf("ジョブの取得のエラー = %v", err)
	}
	defer func() { _ = rows.Close() }()

	locales := []string{}
	for rows.Next() {
		var locale string
		if err := rows.Scan(&locale); err != nil {
			t.Fatalf("ジョブの読み取りのエラー = %v", err)
		}
		locales = append(locales, locale)
	}

	return locales
}

// TestCreatePasswordResetUsecase_Execute は、登録済みのアドレスのときだけ、申請の言語でジョブを投入することを検証する。
// 未登録・退会済みのアドレスでもエラーにしない。
func TestCreatePasswordResetUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc := newCreatePasswordResetUsecase(t)
	db := testutil.GetTestDB()
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	registered := testutil.UniqueEmail("create-password-reset-registered")
	registeredID := testutil.NewUserBuilder(t, db).WithEmail(registered).Build()
	withdrawn := testutil.UniqueEmail("create-password-reset-withdrawn")
	withdrawnID := testutil.NewUserBuilder(t, db).WithEmail(withdrawn).WithDeletedAt(time.Now()).Build()

	tests := []struct {
		name        string
		email       string
		userID      model.UserID
		wantLocales []string
	}{
		{name: "登録済みのアドレス", email: registered, userID: registeredID, wantLocales: []string{"en"}},
		{name: "退会したユーザーのアドレス", email: withdrawn, userID: withdrawnID, wantLocales: []string{}},
	}

	for _, tt := range tests {
		if err := uc.Execute(ctx, usecase.CreatePasswordResetInput{Email: tt.email, Locale: model.LocaleEn}); err != nil {
			t.Fatalf("%s: Execute()のエラー = %v", tt.name, err)
		}

		got := passwordResetJobLocales(t, tt.userID)
		if len(got) != len(tt.wantLocales) || (len(got) == 1 && got[0] != tt.wantLocales[0]) {
			t.Errorf("%s: ジョブの言語 = %v、期待値 = %v", tt.name, got, tt.wantLocales)
		}
	}

	if err := uc.Execute(ctx, usecase.CreatePasswordResetInput{Email: testutil.UniqueEmail("create-password-reset-unknown"), Locale: model.LocaleJa}); err != nil {
		t.Errorf("未登録のアドレス: Execute()のエラー = %v、期待値 = nil", err)
	}
}

// TestCreatePasswordResetUsecase_Execute_Invalid は、形式の誤りをフォームのエラーとして返すことを検証する。
func TestCreatePasswordResetUsecase_Execute_Invalid(t *testing.T) {
	t.Parallel()

	uc := newCreatePasswordResetUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	err := uc.Execute(ctx, usecase.CreatePasswordResetInput{Email: "not-an-email", Locale: model.LocaleJa})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("email") {
		t.Fatalf("Execute()のエラー = %v、emailのフィールドエラーを期待", err)
	}
}
