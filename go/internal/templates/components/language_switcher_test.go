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

// switcherMeta はスイッチャーの検証に使う言語版の顔ぶれを返す。
// currentLocaleの言語版だけがCurrentになる。
func switcherMeta(currentLocale string) viewmodel.PageMeta {
	return viewmodel.PageMeta{
		Alternates: []viewmodel.Alternate{
			{
				Lang:    i18n.LangJa,
				URL:     "https://cutre.example.com/items",
				Endonym: "日本語",
				Current: currentLocale == i18n.LangJa,
			},
			{
				Lang:    i18n.LangEn,
				URL:     "https://cutre.example.com/en/items",
				Endonym: "English",
				Current: currentLocale == i18n.LangEn,
			},
		},
	}
}

// TestLanguageSwitcher は各言語版へのリンクが、その言語自身の名前とlang属性付きで並ぶことを検証する。
// 英語に統一した表記や国旗では、その言語しか読めない利用者が自分の言語を見つけられない。
func TestLanguageSwitcher(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	var buf bytes.Buffer
	if err := components.LanguageSwitcher(switcherMeta(i18n.LangJa)).Render(ctx, &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	html := buf.String()

	wants := []string{
		`aria-label="言語"`,
		`href="https://cutre.example.com/items"`,
		`href="https://cutre.example.com/en/items"`,
		`lang="ja"`,
		`lang="en"`,
		`hreflang="ja"`,
		`hreflang="en"`,
		"日本語",
		"English",
		// クリックを選んだ言語として記録する処理 (web/locale-choice.js) が読む。
		`data-locale-choice="ja"`,
		`data-locale-choice="en"`,
	}
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("出力に %qが含まれていない", want)
		}
	}
}

// TestLanguageSwitcher_AriaCurrent は表示中の言語版だけが選択中として示されることを検証する。
// 視覚的な強調だけでは、支援技術の利用者に現在どの言語版を見ているかが伝わらない。
func TestLanguageSwitcher_AriaCurrent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		locale        string
		wantCurrentIn string
	}{
		{name: "日本語版を表示中", locale: i18n.LangJa, wantCurrentIn: "日本語"},
		{name: "英語版を表示中", locale: i18n.LangEn, wantCurrentIn: "English"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			var buf bytes.Buffer
			if err := components.LanguageSwitcher(switcherMeta(tt.locale)).Render(ctx, &buf); err != nil {
				t.Fatalf("Render()のエラー = %v", err)
			}

			html := buf.String()

			if got := strings.Count(html, `aria-current="true"`); got != 1 {
				t.Errorf("aria-currentの出現回数 = %d、期待値 = 1", got)
			}

			// aria-currentが付いた<a>の開始タグから、そのリンクの文言までを取り出して照合する。
			current := html[strings.Index(html, `aria-current="true"`):]
			current = current[:strings.Index(current, "</a>")]
			if !strings.Contains(current, tt.wantCurrentIn) {
				t.Errorf("aria-currentが付いたリンク = %q、期待値 = %qを含むこと", current, tt.wantCurrentIn)
			}
		})
	}
}

// TestLanguageSwitcher_NoAlternates は言語版を持たないページでスイッチャーを出力しないことを検証する。
// 中身の無いnavは支援技術の領域一覧に名前だけが並び、たどり着いても行き先が無い。
func TestLanguageSwitcher_NoAlternates(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	var buf bytes.Buffer
	if err := components.LanguageSwitcher(viewmodel.PageMeta{}).Render(ctx, &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Errorf("出力 = %q、期待値 = 空文字列", got)
	}
}
