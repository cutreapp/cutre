package validator_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestAccountCreateValidator_Validate は、アットネームとパスワードの形式の誤りと、アットネーム・メールアドレスの重なりを
// それぞれのエラーにすることを検証する。
func TestAccountCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewAccountCreateValidator(repository.NewUserRepository(db).WithTx(tx))

	takenAtname := testutil.UniqueAtname()
	takenEmail := testutil.UniqueEmail("account-taken")
	testutil.NewUserBuilder(t, tx).WithAtname(takenAtname).WithEmail(takenEmail).Build()
	withdrawnAtname := testutil.UniqueAtname()
	testutil.NewUserBuilder(t, tx).WithAtname(withdrawnAtname).WithDeletedAt(time.Now()).Build()

	const password = "password1234"
	tests := []struct {
		name          string
		email         string
		atname        string
		password      string
		wantAtnameErr string
		wantPassErr   string
		wantGlobalErr string
	}{
		{name: "使える入力", atname: testutil.UniqueAtname(), password: password},
		{name: "20文字のアットネーム", atname: "a" + strings.Repeat("_", 19), password: password},
		{name: "退会したユーザーのアットネーム", atname: withdrawnAtname, password: password},
		{name: "8文字の日本語のパスワード", atname: testutil.UniqueAtname(), password: "あいうえおかきく"},
		{name: "72バイトのパスワード", atname: testutil.UniqueAtname(), password: strings.Repeat("a", 72)},
		{name: "アットネームの未入力", atname: "", password: password, wantAtnameErr: "入力してください"},
		{name: "21文字のアットネーム", atname: strings.Repeat("a", 21), password: password, wantAtnameErr: "20文字以内で入力してください"},
		{name: "使えない文字", atname: "cutre-user", password: password, wantAtnameErr: "半角英数字とアンダースコア (_) だけで入力してください"},
		{name: "全角の文字", atname: "くとれ", password: password, wantAtnameErr: "半角英数字とアンダースコア (_) だけで入力してください"},
		{name: "使われているアットネーム (大文字小文字の違い)", atname: strings.ToUpper(takenAtname), password: password, wantAtnameErr: "このアットネームは既に使われています"},
		{name: "パスワードの未入力", atname: testutil.UniqueAtname(), password: "", wantPassErr: "入力してください"},
		{name: "7文字のパスワード", atname: testutil.UniqueAtname(), password: "abcdefg", wantPassErr: "8文字以上で入力してください"},
		{name: "73バイトのパスワード", atname: testutil.UniqueAtname(), password: strings.Repeat("a", 73), wantPassErr: "長すぎます"},
		{name: "登録済みのメールアドレス", email: takenEmail, atname: testutil.UniqueAtname(), password: password, wantGlobalErr: "このメールアドレスのアカウントは既にあります"},
	}

	for _, tt := range tests {
		email := tt.email
		if email == "" {
			email = testutil.UniqueEmail("account-new")
		}
		err := v.Validate(ctx, validator.AccountCreateValidatorInput{Email: email, Atname: tt.atname, Password: tt.password})

		if tt.wantAtnameErr == "" && tt.wantPassErr == "" && tt.wantGlobalErr == "" {
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
		assertMessage(t, tt.name+" (atname)", ve.GetFieldErrors("atname"), tt.wantAtnameErr)
		assertMessage(t, tt.name+" (password)", ve.GetFieldErrors("password"), tt.wantPassErr)
		assertMessage(t, tt.name+" (global)", ve.Global, tt.wantGlobalErr)
	}
}

// TestAccountCreateValidator_Validate_FormatBeforeLookup は、形式の誤りがあるときは重なりを確かめないことを検証する。
// 使われているアットネームでも、パスワードが短ければそのエラーだけを返す。
func TestAccountCreateValidator_Validate_FormatBeforeLookup(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewAccountCreateValidator(repository.NewUserRepository(db).WithTx(tx))
	takenAtname := testutil.UniqueAtname()
	testutil.NewUserBuilder(t, tx).WithAtname(takenAtname).Build()

	err := v.Validate(ctx, validator.AccountCreateValidatorInput{Email: testutil.UniqueEmail("account-format"), Atname: takenAtname, Password: "short"})

	ve := model.AsValidationError(err)
	if ve == nil || !ve.HasFieldError("password") || ve.HasFieldError("atname") {
		t.Errorf("エラー = %+v、passwordだけのフィールドエラーを期待", ve)
	}
}

// assertMessage は、wantが空ならメッセージが無いことを、そうでなければwantを含むメッセージがあることを確かめる。
func assertMessage(t *testing.T, name string, got []string, want string) {
	t.Helper()

	if want == "" {
		if len(got) > 0 {
			t.Errorf("%s: エラー = %v、無しを期待", name, got)
		}
		return
	}
	if len(got) != 1 || !strings.Contains(got[0], want) {
		t.Errorf("%s: エラー = %v、%q を含む1件を期待", name, got, want)
	}
}
