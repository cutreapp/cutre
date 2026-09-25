package layouts_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/cutreapp/cutre/go/internal/templates/emails/layouts"
)

// TestDefault は、引数の言語と件名で文書の外枠を描き、各メールの本文とフッターを並べることを検証する。
func TestDefault(t *testing.T) {
	t.Parallel()

	ctx := templ.WithChildren(context.Background(), templ.Raw("<p>本文</p>"))

	var buf bytes.Buffer
	if err := layouts.Default("en", "Confirmation code").Render(ctx, &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	html := buf.String()
	wants := []string{
		"<!doctype html>",
		`<html lang="en">`,
		`<meta charset="utf-8">`,
		"<title>Confirmation code</title>",
		"<p>本文</p>",
		">Cutre</p>",
	}
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("出力に %q が含まれていない", want)
		}
	}
}
