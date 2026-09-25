package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/templates/components"
)

// TestTurnstile は、サイトキーがあればスクリプトとウィジェットを描画し、
// 無ければ (Turnstileを無効にした環境) 外部のスクリプトを含めて何も出力しないことを検証する。
func TestTurnstile(t *testing.T) {
	t.Parallel()

	// Cloudflareが提供する、常に通過するテスト用のサイトキー。
	const dummySiteKey = "1x00000000000000000000AA"

	tests := []struct {
		name         string
		siteKey      string
		locale       string
		wantContains []string
	}{
		{
			name:    "サイトキーがあればスクリプトとウィジェットを描画する",
			siteKey: dummySiteKey,
			locale:  i18n.LangJa,
			wantContains: []string{
				`<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>`,
				`class="cf-turnstile"`,
				`data-sitekey="1x00000000000000000000AA"`,
				`data-theme="auto"`,
				`data-language="ja"`,
			},
		},
		{
			name:         "ウィジェットの言語は表示中のページに合わせる",
			siteKey:      dummySiteKey,
			locale:       i18n.LangEn,
			wantContains: []string{`data-language="en"`},
		},
		{
			name:    "サイトキーが空なら何も出力しない",
			siteKey: "",
			locale:  i18n.LangJa,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			var buf strings.Builder
			if err := components.Turnstile(tt.siteKey).Render(ctx, &buf); err != nil {
				t.Fatalf("描画のエラー = %v", err)
			}
			got := buf.String()

			if tt.siteKey == "" {
				if strings.TrimSpace(got) != "" {
					t.Errorf("出力 = %q、空を期待", got)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
		})
	}
}
