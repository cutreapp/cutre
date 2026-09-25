package validator_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/validator"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// TestSignInCreateValidator_Validate は、資格情報が一致すればユーザーを返し、
// 一致しない理由がどれであっても同じ1つのメッセージを返すことを検証する。
func TestSignInCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	const password = "password123"
	const credentialsInvalid = "メールアドレスまたはパスワードが正しくありません"

	tests := []struct {
		name string
		// setup はユーザーを用意し、送信するメールアドレスを返す。
		setup      func(t *testing.T, db *sql.Tx) string
		password   string
		wantUser   bool
		wantGlobal string
	}{
		{
			name: "資格情報が一致すればユーザーを返す",
			setup: func(t *testing.T, db *sql.Tx) string {
				email := testutil.UniqueEmail("sign-in")
				userID := testutil.NewUserBuilder(t, db).WithEmail(email).Build()
				testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword(password).Build()
				return email
			},
			password: password,
			wantUser: true,
		},
		{
			name: "メールアドレスは大文字小文字を区別せずに照合する",
			setup: func(t *testing.T, db *sql.Tx) string {
				email := testutil.UniqueEmail("sign-in")
				userID := testutil.NewUserBuilder(t, db).WithEmail(email).Build()
				testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword(password).Build()
				return strings.ToUpper(email)
			},
			password: password,
			wantUser: true,
		},
		{
			name: "パスワードが違う",
			setup: func(t *testing.T, db *sql.Tx) string {
				email := testutil.UniqueEmail("sign-in")
				userID := testutil.NewUserBuilder(t, db).WithEmail(email).Build()
				testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword(password).Build()
				return email
			},
			password:   "password124",
			wantGlobal: credentialsInvalid,
		},
		{
			name: "未知のメールアドレス",
			setup: func(t *testing.T, _ *sql.Tx) string {
				return testutil.UniqueEmail("unknown")
			},
			password:   password,
			wantGlobal: credentialsInvalid,
		},
		{
			name: "パスワードを持たないアカウント",
			setup: func(t *testing.T, db *sql.Tx) string {
				email := testutil.UniqueEmail("sign-in")
				testutil.NewUserBuilder(t, db).WithEmail(email).Build()
				return email
			},
			password:   password,
			wantGlobal: credentialsInvalid,
		},
		{
			name: "退会したユーザー",
			setup: func(t *testing.T, db *sql.Tx) string {
				email := testutil.UniqueEmail("sign-in")
				userID := testutil.NewUserBuilder(t, db).WithEmail(email).WithDeletedAt(time.Now()).Build()
				testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword(password).Build()
				return email
			},
			password:   password,
			wantGlobal: credentialsInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
			v := validator.NewSignInCreateValidator(
				repository.NewUserRepository(db).WithTx(tx),
				repository.NewUserPasswordRepository(db).WithTx(tx),
			)

			email := tt.setup(t, tx)

			user, err := v.Validate(ctx, validator.SignInCreateValidatorInput{Email: email, Password: tt.password})

			if tt.wantUser {
				if err != nil {
					t.Fatalf("Validate()のエラー = %v", err)
				}
				if user == nil {
					t.Fatal("ユーザー = nil、非nilを期待")
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("エラー = %v、ValidationErrorを期待", err)
			}
			if user != nil {
				t.Errorf("ユーザー = %+v、nilを期待", user)
			}
			if len(ve.Global) != 1 || ve.Global[0] != tt.wantGlobal {
				t.Errorf("Global = %v、期待値 = [%q]", ve.Global, tt.wantGlobal)
			}
			// どの理由で一致しなかったかを、フィールドのエラーから読み取れてはならない。
			if len(ve.Fields) != 0 {
				t.Errorf("Fields = %v、空を期待", ve.Fields)
			}
		})
	}
}

// TestSignInCreateValidator_Validate_Format は、形式の誤りをデータベースを引く前にフィールドのエラーとして返すことを検証する。
func TestSignInCreateValidator_Validate_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      validator.SignInCreateValidatorInput
		wantFields map[string]string
	}{
		{
			name:  "両方が空",
			input: validator.SignInCreateValidatorInput{},
			wantFields: map[string]string{
				"email":    "入力してください",
				"password": "入力してください",
			},
		},
		{
			name:       "メールアドレスとして読めない",
			input:      validator.SignInCreateValidatorInput{Email: "not-an-email", Password: "password123"},
			wantFields: map[string]string{"email": "メールアドレスの形式が正しくありません"},
		},
		{
			name:       "表示名付きの形は受け付けない",
			input:      validator.SignInCreateValidatorInput{Email: "Name <name@example.com>", Password: "password123"},
			wantFields: map[string]string{"email": "メールアドレスの形式が正しくありません"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 形式の誤りではデータベースを引かないため、リポジトリにnilの接続を渡しても通る。
			v := validator.NewSignInCreateValidator(repository.NewUserRepository(nil), repository.NewUserPasswordRepository(nil))
			ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

			_, err := v.Validate(ctx, tt.input)

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("エラー = %v、ValidationErrorを期待", err)
			}
			if len(ve.Fields) != len(tt.wantFields) {
				t.Errorf("Fields = %v、期待値 = %v", ve.Fields, tt.wantFields)
			}
			for field, want := range tt.wantFields {
				if got := ve.GetFieldErrors(field); len(got) != 1 || got[0] != want {
					t.Errorf("%sのエラー = %v、期待値 = [%q]", field, got, want)
				}
			}
		})
	}
}
