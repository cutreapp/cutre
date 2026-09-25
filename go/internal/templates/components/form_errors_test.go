package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates/components"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()

	var buf strings.Builder
	if err := component.Render(i18n.SetLocale(context.Background(), i18n.LangJa), &buf); err != nil {
		t.Fatalf("描画のエラー = %v", err)
	}
	return buf.String()
}

// TestFieldErrors は、エラーメッセージの要素のidが FieldErrorsDescribedBy の返す値と揃い、
// 入力欄の aria-describedby から参照できることを検証する。
func TestFieldErrors(t *testing.T) {
	t.Parallel()

	ve := model.NewValidationError()
	ve.AddField("email", "入力してください")
	ve.AddField("email", "形式が正しくありません")

	got := render(t, components.FieldErrors("email", ve))

	if describedBy := components.FieldErrorsDescribedBy("email", ve); describedBy != "email-error-0 email-error-1" {
		t.Errorf("FieldErrorsDescribedBy() = %q、期待値 = %q", describedBy, "email-error-0 email-error-1")
	}
	for _, want := range []string{
		`<p id="email-error-0" role="alert">入力してください</p>`,
		`<p id="email-error-1" role="alert">形式が正しくありません</p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}

	// エラーの無い入力欄は、参照先の無い aria-describedby を持たないよう空を返す。
	if describedBy := components.FieldErrorsDescribedBy("password", ve); describedBy != "" {
		t.Errorf("FieldErrorsDescribedBy() = %q、空を期待", describedBy)
	}
	if got := render(t, components.FieldErrors("email", nil)); got != "" {
		t.Errorf("エラーが無いときの出力 = %q、空を期待", got)
	}
}

// TestFormErrorSummary は、フィールドのエラーをラベル付きで並べ、各項目が入力欄へのリンクになることと、
// エラーが無ければ何も出力しないことを検証する。
func TestFormErrorSummary(t *testing.T) {
	t.Parallel()

	fields := []components.FormErrorSummaryField{
		{Name: "email", LabelKey: "sign_in_new_email_label"},
		{Name: "password", LabelKey: "sign_in_new_password_label"},
	}

	ve := model.NewValidationError()
	ve.AddField("password", "入力してください")

	got := render(t, components.FormErrorSummary(components.FormErrorSummaryData{Fields: fields, Errors: ve}))
	for _, want := range []string{"入力内容を確認してください", `href="#password"`, "パスワード: 入力してください", `tabindex="-1" autofocus`} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}
	// 同じメッセージを入力欄の近くでも読み上げるため、要約は通知しない。
	if strings.Contains(got, `role="alert"`) {
		t.Errorf("要約に role=\"alert\" が付いている\n出力: %s", got)
	}

	// フォーム全体のエラーだけのときは、要約に並べるものが無い。
	global := model.NewValidationError()
	global.AddGlobal("メールアドレスまたはパスワードが正しくありません")
	if got := render(t, components.FormErrorSummary(components.FormErrorSummaryData{Fields: fields, Errors: global})); got != "" {
		t.Errorf("フィールドのエラーが無いときの出力 = %q、空を期待", got)
	}
}

// TestFormErrors は、フォーム全体のエラーを通知の対象として描画することを検証する。
func TestFormErrors(t *testing.T) {
	t.Parallel()

	ve := model.NewValidationError()
	ve.AddGlobal("メールアドレスまたはパスワードが正しくありません")
	ve.AddGlobal("2件目のエラー")

	got := render(t, components.FormErrors(ve))
	if !strings.Contains(got, `role="alert"`) || !strings.Contains(got, "メールアドレスまたはパスワードが正しくありません") {
		t.Errorf("出力 = %s、role=\"alert\" 付きのメッセージを期待", got)
	}
	// フォーカスを移す先は最初のエラーだけにする。
	if count := strings.Count(got, "autofocus"); count != 1 {
		t.Errorf("autofocus の数 = %d、期待値 = 1\n出力: %s", count, got)
	}
	if got := render(t, components.FormErrors(nil)); got != "" {
		t.Errorf("エラーが無いときの出力 = %q、空を期待", got)
	}
}

// TestRequiredFieldLabel は、ラベルが入力欄を指し、必須であることを文字で添えることを検証する。
func TestRequiredFieldLabel(t *testing.T) {
	t.Parallel()

	got := render(t, components.RequiredFieldLabel("email", "メールアドレス"))
	for _, want := range []string{`for="email"`, "メールアドレス", "(必須)"} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}
}
