package layouts_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestDefault はレイアウトが標準モードのHTMLとして成立し、本文の前にスキップリンクを、
// 飛び先として<main id="main">を持つことを検証する (WCAG 2.4.1)。
func TestDefault(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	data := layouts.DefaultLayoutData{
		Meta: viewmodel.PageMeta{Title: "テストページ", Description: "説明", AssetVersion: "abc1234"},
	}
	content := templ.Raw("<p>本文</p>")

	var buf bytes.Buffer
	if err := layouts.Default(data, content).Render(ctx, &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	html := buf.String()

	if !strings.HasPrefix(html, "<!doctype html>") {
		t.Errorf("出力の先頭 = %q、期待値 = <!doctype html>で始まること", html[:min(len(html), 30)])
	}

	wants := []string{
		`<html lang="ja">`,
		`<a href="#main"`,
		"メインコンテンツへスキップ",
		`<main id="main" tabindex="-1"`,
		"<p>本文</p>",
		"<footer",
		"&copy; 2026 Cutre",
	}
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("出力に %qが含まれていない", want)
		}
	}

	// スキップリンクは本文より前に置かれて初めて本文を飛ばす手段になる。
	if strings.Index(html, `<a href="#main"`) > strings.Index(html, `<main id="main"`) {
		t.Error("スキップリンクが<main>より後ろに出力されている")
	}
}

// TestDefault_Locale は表示中のロケールがlang属性に反映されることを検証する。
// テンプレートに固定値を書くと、英語のページを日本語として読み上げさせることになる (WCAG 3.1.1)。
func TestDefault_Locale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		locale   string
		wantLang string
	}{
		{name: "日本語", locale: i18n.LangJa, wantLang: `<html lang="ja">`},
		{name: "英語", locale: i18n.LangEn, wantLang: `<html lang="en">`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			var buf bytes.Buffer
			if err := layouts.Default(layouts.DefaultLayoutData{}, templ.NopComponent).Render(ctx, &buf); err != nil {
				t.Fatalf("Render()のエラー = %v", err)
			}

			if !strings.Contains(buf.String(), tt.wantLang) {
				t.Errorf("出力に %qが含まれていない", tt.wantLang)
			}
		})
	}
}

// TestDefault_LanguageVersions は言語スイッチャーがフッターに常設され、
// 言語版の案内が本文より前に出ることを検証する。
//
// 案内を本文より後ろに置くと、別の言語版を探している利用者が読めない本文を越えるまで気付けない。
func TestDefault_LanguageVersions(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	data := layouts.DefaultLayoutData{
		Meta: viewmodel.PageMeta{
			Alternates: []viewmodel.Alternate{
				{Lang: i18n.LangJa, URL: "https://cutre.example.com/", Endonym: "日本語", Current: true},
				{Lang: i18n.LangEn, URL: "https://cutre.example.com/en", Endonym: "English"},
			},
			LocaleSuggestion: &viewmodel.LocaleSuggestion{
				Lang:         i18n.LangEn,
				URL:          "https://cutre.example.com/en",
				Label:        "Language notice",
				Message:      "This page is also available in English.",
				SwitchLabel:  "Read in English",
				DismissLabel: "Dismiss",
			},
		},
	}

	var buf bytes.Buffer
	if err := layouts.Default(data, templ.Raw("<p>本文</p>")).Render(ctx, &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	html := buf.String()

	if strings.Index(html, "data-locale-suggestion") > strings.Index(html, `<main id="main"`) {
		t.Error("言語版の案内が<main>より後ろに出力されている")
	}

	footer := html[strings.Index(html, "<footer"):]
	if !strings.Contains(footer, "English") {
		t.Error("フッターに言語スイッチャーが出力されていない")
	}
}
