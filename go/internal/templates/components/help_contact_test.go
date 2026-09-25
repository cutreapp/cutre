package components_test

import (
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates/components"
)

// TestHelpContact は、ヘルプのお問い合わせのページを新しいタブで開くリンクを置き、
// 新しいタブで開くことを伝える補足をリンクの説明に結び付けることを検証する。
func TestHelpContact(t *testing.T) {
	t.Parallel()

	got := render(t, components.HelpContact())

	for _, want := range []string{
		"認証アプリもリカバリーコードもないときは",
		`href="https://wikino.app/s/cutre"`,
		`target="_blank"`,
		`rel="noopener"`,
		`aria-describedby="help-contact-new-tab"`,
		"ヘルプからお問い合わせ",
		`<p id="help-contact-new-tab" class="text-xs text-muted-foreground">ヘルプのページ (Wikino) が開きます</p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}
}
