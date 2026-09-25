package components_test

import (
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
)

// TestHeader は、ヘッダーがホームと自分のプロフィールへのリンクを持ち、表示中のページを指すリンクにだけ
// aria-current を付けることを検証する。
func TestHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		currentPath string
		want        []string
		absent      []string
	}{
		{
			name:        "ホームを表示中",
			currentPath: templates.HomePath,
			want:        []string{`href="/home" class="text-2xl font-bold tracking-tight" aria-current="page"`, `<nav aria-label="メインメニュー">`, `href="/@cutre_user"`, "プロフィール"},
			absent:      []string{`data-size="sm" aria-current="page"`},
		},
		{
			name:        "プロフィールを表示中",
			currentPath: "/@cutre_user",
			want:        []string{`data-size="sm" aria-current="page"`},
			absent:      []string{`tracking-tight" aria-current="page"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := render(t, components.Header(components.HeaderData{Atname: "cutre_user", CurrentPath: tt.currentPath}))

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
