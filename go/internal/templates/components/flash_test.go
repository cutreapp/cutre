package components_test

import (
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/templates/components"
)

// TestFlash は、種類ごとの通知の強さと自動で閉じるかどうか、そしてメッセージが無ければ何も出力しないことを検証する。
func TestFlash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		flash        *session.FlashMessage
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:  "成功は読み上げを遮らずに伝え、既定の時間で閉じる",
			flash: &session.FlashMessage{Type: session.FlashSuccess, Message: "ログインしました"},
			wantContains: []string{
				`class="toaster"`,
				`role="status"`,
				`data-category="success"`,
				"ログインしました",
				"閉じる",
			},
			wantAbsent: []string{`role="alert"`, "data-duration"},
		},
		{
			name:         "失敗は直ちに読み上げ、自動では閉じない",
			flash:        &session.FlashMessage{Type: session.FlashError, Message: "失敗しました"},
			wantContains: []string{`role="alert"`, `data-duration="-1"`, `data-category="error"`},
			wantAbsent:   []string{`role="status"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := render(t, components.Flash(tt.flash))
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("出力に %q が含まれている\n出力: %s", absent, got)
				}
			}
		})
	}

	if got := render(t, components.Flash(nil)); got != "" {
		t.Errorf("メッセージが無いときの出力 = %q、空を期待", got)
	}
}
