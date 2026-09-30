package components_test

import (
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates/components"
)

// TestPageHeading は、メインメニューの行き先の画面ではタイトルだけを、それ以外の画面ではパンくずとタイトルを出し、
// パンくずの最後に今のページを aria-current="page" で置くことを検証する。
func TestPageHeading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   components.PageHeadingData
		want   []string
		absent []string
	}{
		{
			name: "タイトルだけ",
			data: components.PageHeadingData{Title: "ホーム"},
			want: []string{`<h1 class="text-2xl font-bold">ホーム</h1>`},
			absent: []string{
				"<nav",
				"aria-current",
			},
		},
		{
			name: "パンくずとタイトル",
			data: components.PageHeadingData{
				Title: "二要素認証を有効にする",
				Breadcrumbs: []components.Breadcrumb{
					{Label: "マイページ", Path: "/@cutre_user"},
					{Label: "二要素認証", Path: "/settings/two_factor_auth"},
				},
			},
			want: []string{
				`<nav aria-label="パンくずリスト">`,
				`href="/@cutre_user"`,
				"マイページ",
				`href="/settings/two_factor_auth"`,
				"二要素認証",
				`<li class="sr-only" aria-current="page">二要素認証を有効にする</li>`,
				`<h1 class="text-xl font-semibold">二要素認証を有効にする</h1>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := render(t, components.PageHeading(tt.data))

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(got, absent) {
					t.Errorf("出力に %q が含まれている\n出力: %s", absent, got)
				}
			}
		})
	}
}

// TestPageHeading_BreadcrumbOrder は、パンくずを渡した順に並べ、タイトルをパンくずの後に置くことを検証する。
func TestPageHeading_BreadcrumbOrder(t *testing.T) {
	t.Parallel()

	got := render(t, components.PageHeading(components.PageHeadingData{
		Title: "二要素認証を有効にする",
		Breadcrumbs: []components.Breadcrumb{
			{Label: "マイページ", Path: "/@cutre_user"},
			{Label: "二要素認証", Path: "/settings/two_factor_auth"},
		},
	}))

	first := strings.Index(got, `href="/@cutre_user"`)
	second := strings.Index(got, `href="/settings/two_factor_auth"`)
	heading := strings.Index(got, "<h1")
	if first >= second || second >= heading {
		t.Errorf("パンくずとタイトルの順序が正しくない (マイページ: %d、二要素認証: %d、タイトル: %d)\n出力: %s", first, second, heading, got)
	}
}
