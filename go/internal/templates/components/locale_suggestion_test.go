package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestLocaleSuggestion は案内すべき言語版があるときだけ案内が出ることを検証する。
// 求めている言語版を既に見ている利用者に案内を出すと、閉じる手間だけが残る。
func TestLocaleSuggestion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		meta           viewmodel.PageMeta
		wantSuggestion bool
	}{
		{
			name: "案内すべき言語版があるとき",
			meta: viewmodel.PageMeta{LocaleSuggestion: &viewmodel.LocaleSuggestion{
				Lang:          i18n.LangJa,
				URL:           "https://cutre.example.com/",
				Label:         "言語版の案内",
				Message:       "このページは日本語でも読めます。",
				SwitchLabel:   "日本語で読む",
				DismissLabel:  "閉じる",
				DismissLocale: i18n.LangEn,
			}},
			wantSuggestion: true,
		},
		{
			name:           "案内すべき言語版が無いとき",
			meta:           viewmodel.PageMeta{},
			wantSuggestion: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := components.LocaleSuggestion(tt.meta).Render(context.Background(), &buf); err != nil {
				t.Fatalf("Render()のエラー = %v", err)
			}

			if got := strings.Contains(buf.String(), "data-locale-suggestion"); got != tt.wantSuggestion {
				t.Errorf("案内の出力有無 = %t、期待値 = %t", got, tt.wantSuggestion)
			}
		})
	}
}

// TestLocaleSuggestion_Contents は案内先の言語の文言とリンク先、閉じるボタンが揃うことを検証する。
// lang属性は文言の言語を宣言する。表示中のページの言語のままだと、案内だけが別の言語で読み上げられる。
func TestLocaleSuggestion_Contents(t *testing.T) {
	t.Parallel()

	meta := viewmodel.PageMeta{LocaleSuggestion: &viewmodel.LocaleSuggestion{
		Lang:          i18n.LangEn,
		URL:           "https://cutre.example.com/en",
		Label:         "Language notice",
		Message:       "This page is also available in English.",
		SwitchLabel:   "Read in English",
		DismissLabel:  "Dismiss",
		DismissLocale: i18n.LangJa,
	}}

	var buf bytes.Buffer
	if err := components.LocaleSuggestion(meta).Render(context.Background(), &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	html := buf.String()

	wants := []string{
		`lang="en"`,
		// ランドマークの名前も案内先の言語で出す。
		`aria-label="Language notice"`,
		`hreflang="en"`,
		`href="https://cutre.example.com/en"`,
		"This page is also available in English.",
		"Read in English",
		// 移動リンクは案内先の言語を選んだものとして記録する。
		`data-locale-choice="en"`,
		// 閉じる操作はその場で完結するためボタンにする。
		`<button type="button"`,
		"data-locale-suggestion-dismiss",
		// 閉じる操作は表示中の言語版を選んだものとして記録する。
		`data-locale-choice="ja"`,
		"Dismiss",
	}
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("出力に %qが含まれていない", want)
		}
	}
}
