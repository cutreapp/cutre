package components_test

import (
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates/components"
)

// TestAuthHeading は、見出しの上にトップへ戻るロゴのリンクを置き、リンクの名前に見えている文字列を含めることを検証する。
func TestAuthHeading(t *testing.T) {
	t.Parallel()

	got := render(t, components.AuthHeading("ログイン"))

	for _, want := range []string{
		`href="/"`,
		`aria-label="Cutreのトップへ"`,
		`<h1 class="text-2xl font-bold">ログイン</h1>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}
}
