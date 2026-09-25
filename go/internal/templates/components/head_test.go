package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestHead_CanonicalURL は正規のアドレスを持つページだけがそれを宣言することを検証する。
// 空のhref / contentはリクエストされたURL自身を指すため、値を持たないページで出力すると
// クエリ違いのURLがそれぞれ自分を正規のアドレスとして名乗ってしまう。
func TestHead_CanonicalURL(t *testing.T) {
	t.Parallel()

	const canonical = "https://cutre.example.com/"

	tests := []struct {
		name          string
		meta          viewmodel.PageMeta
		wantCanonical bool
	}{
		{
			name:          "正規のアドレスを持つページは宣言する",
			meta:          viewmodel.PageMeta{CanonicalURL: canonical},
			wantCanonical: true,
		},
		{
			name:          "正規のアドレスを持たないページは宣言しない",
			meta:          viewmodel.PageMeta{},
			wantCanonical: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			var buf bytes.Buffer
			if err := components.Head(tt.meta).Render(ctx, &buf); err != nil {
				t.Fatalf("Render()のエラー = %v", err)
			}

			html := buf.String()
			for _, tag := range []string{`<link rel="canonical"`, `<meta property="og:url"`} {
				if got := strings.Contains(html, tag); got != tt.wantCanonical {
					t.Errorf("%qの出力有無 = %t、期待値 = %t", tag, got, tt.wantCanonical)
				}
			}
		})
	}
}

// TestHead_MetaCharset は文字エンコーディングの宣言が<head>の先頭に来ることを検証する。
// HTML Standardはこの宣言が先頭1024バイト以内にあることを求めており、
// 前に長いタグが挟まるとブラウザがエンコーディングを推測し直す。
func TestHead_MetaCharset(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var buf bytes.Buffer
	if err := components.Head(viewmodel.PageMeta{Title: "テスト"}).Render(ctx, &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	html := buf.String()
	if !strings.HasPrefix(html, `<meta charset="utf-8">`) {
		t.Errorf("出力の先頭 = %q、期待値 = meta charsetで始まること", html[:min(len(html), 40)])
	}

	wants := []string{
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		`<title>テスト</title>`,
		`<meta name="color-scheme" content="light dark">`,
		`<script src="/static/js/theme.js?v="></script>`,
		`<meta property="og:type" content="website">`,
	}
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("出力に %qが含まれていない", want)
		}
	}
}

// TestHead_Alternates は全言語版が自己参照を含めて出力されることを検証する。
// 自己参照が欠けた片方向のアノテーションは検索エンジンに無視されるため、
// 表示中の言語版の分も落とさずに出す必要がある。
func TestHead_Alternates(t *testing.T) {
	t.Parallel()

	meta := viewmodel.PageMeta{
		CanonicalURL: "https://cutre.example.com/en",
		Alternates: []viewmodel.Alternate{
			{Lang: "ja", URL: "https://cutre.example.com/"},
			{Lang: "en", URL: "https://cutre.example.com/en"},
		},
	}

	var buf bytes.Buffer
	if err := components.Head(meta).Render(context.Background(), &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	html := buf.String()
	wants := []string{
		`<link rel="alternate" hreflang="ja" href="https://cutre.example.com/">`,
		`<link rel="alternate" hreflang="en" href="https://cutre.example.com/en">`,
	}
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("出力に %qが含まれていない", want)
		}
	}
}

// TestHead_NoAlternates は言語版を持たないページがhreflangを出さないことを検証する。
// 相互参照する相手がいない状態でアノテーションを出しても意味を持たない。
func TestHead_NoAlternates(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := components.Head(viewmodel.PageMeta{}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	if strings.Contains(buf.String(), `rel="alternate"`) {
		t.Error("言語版を持たないページの出力に rel=\"alternate\" が含まれている")
	}
}

// TestHead_NoIndex は、インデックスを断るページだけがrobotsのmetaを出し、
// そのページではcanonicalを宣言しないことを検証する。
// 「正規はこのアドレス」と「インデックスするな」を同時に出すと、検索エンジンに矛盾したシグナルを送る。
func TestHead_NoIndex(t *testing.T) {
	t.Parallel()

	const canonical = "https://cutre.example.com/sign_in"

	tests := []struct {
		name          string
		meta          viewmodel.PageMeta
		wantNoIndex   bool
		wantCanonical bool
	}{
		{
			name:          "インデックスを許すページはrobotsのmetaを出さない",
			meta:          viewmodel.PageMeta{CanonicalURL: canonical},
			wantNoIndex:   false,
			wantCanonical: true,
		},
		{
			name:          "インデックスを断るページはrobotsのmetaを出し、canonicalを宣言しない",
			meta:          viewmodel.PageMeta{CanonicalURL: canonical, NoIndex: true},
			wantNoIndex:   true,
			wantCanonical: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := components.Head(tt.meta).Render(context.Background(), &buf); err != nil {
				t.Fatalf("Render()のエラー = %v", err)
			}

			html := buf.String()
			if got := strings.Contains(html, `<meta name="robots" content="noindex">`); got != tt.wantNoIndex {
				t.Errorf("robotsのmetaの出力有無 = %t、期待値 = %t", got, tt.wantNoIndex)
			}
			for _, tag := range []string{`<link rel="canonical"`, `<meta property="og:url"`} {
				if got := strings.Contains(html, tag); got != tt.wantCanonical {
					t.Errorf("%qの出力有無 = %t、期待値 = %t", tag, got, tt.wantCanonical)
				}
			}
		})
	}
}
