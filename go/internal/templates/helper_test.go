package templates_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/templates"
)

// TestT はヘルパーがctxのロケールで翻訳を返すことを検証する。
func TestT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{name: "日本語", locale: i18n.LangJa, want: "メインコンテンツへスキップ"},
		{name: "英語", locale: i18n.LangEn, want: "Skip to main content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			if got := templates.T(ctx, "skip_to_main_content"); got != tt.want {
				t.Errorf("T() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestLocale はヘルパーがctxのロケールを返し、未設定なら既定のロケールになることを検証する。
func TestLocale(t *testing.T) {
	t.Parallel()

	if got := templates.Locale(i18n.SetLocale(context.Background(), i18n.LangEn)); got != i18n.LangEn {
		t.Errorf("Locale() = %q、期待値 = %q", got, i18n.LangEn)
	}

	if got := templates.Locale(context.Background()); got != i18n.DefaultLang {
		t.Errorf("ロケール未設定のLocale() = %q、期待値 = %q", got, i18n.DefaultLang)
	}
}

// TestRootPath はトップページのパスが、ctxのロケールの言語版を指すことを検証する。
func TestRootPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{name: "日本語", locale: i18n.LangJa, want: "/"},
		{name: "英語", locale: i18n.LangEn, want: "/en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			if got := templates.RootPath(ctx); got != tt.want {
				t.Errorf("RootPath() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestInvitationPath は、招待リンクのパスが /i/ の後ろにトークンを置く短い形になることを検証する。
func TestInvitationPath(t *testing.T) {
	t.Parallel()

	if got := templates.InvitationPath("abc_DEF-123"); got != "/i/abc_DEF-123" {
		t.Errorf("InvitationPath() = %q、期待値 = %q", got, "/i/abc_DEF-123")
	}
}

// TestWithReturnTo は、戻り先をクエリの値としてエスケープして付け、空なら何も付けないことを検証する。
func TestWithReturnTo(t *testing.T) {
	t.Parallel()

	if got, want := templates.WithReturnTo("/sign_in/two_factor", "/@cutre_user?tab=1"), "/sign_in/two_factor?return_to=%2F%40cutre_user%3Ftab%3D1"; got != want {
		t.Errorf("WithReturnTo() = %q、期待値 = %q", got, want)
	}
	if got, want := templates.WithReturnTo("/sign_in", ""), "/sign_in"; got != want {
		t.Errorf("WithReturnTo() = %q、期待値 = %q", got, want)
	}
}
