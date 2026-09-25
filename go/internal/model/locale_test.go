package model_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestParseLocale は、翻訳を持つ言語だけが Locale として通ることを検証する。
func TestParseLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  model.Locale
		wasOK bool
	}{
		{name: "日本語のタグ", input: "ja", want: model.LocaleJa, wasOK: true},
		{name: "英語のタグ", input: "en", want: model.LocaleEn, wasOK: true},
		{name: "翻訳を持たない言語のタグ", input: "fr", want: "", wasOK: false},
		{name: "大文字のタグ", input: "JA", want: "", wasOK: false},
		{name: "空文字列", input: "", want: "", wasOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ParseLocale(tt.input)
			if ok != tt.wasOK {
				t.Errorf("ParseLocale(%q)の2つ目の戻り値 = %t、期待値 = %t", tt.input, ok, tt.wasOK)
			}

			if got != tt.want {
				t.Errorf("ParseLocale(%q) = %q、期待値 = %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestLocales_Copy は、返したスライスへの変更が次の呼び出しに波及しないことを検証する。
func TestLocales_Copy(t *testing.T) {
	t.Parallel()

	locales := model.Locales()
	if len(locales) == 0 {
		t.Fatal("Locales() = 空、非空を期待")
	}

	locales[0] = "fr"

	if got := model.Locales()[0]; got == "fr" {
		t.Error("Locales()の戻り値への変更が次の呼び出しに波及している")
	}
}
