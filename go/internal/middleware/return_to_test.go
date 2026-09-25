package middleware_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestSanitizeReturnTo は、ログイン後の戻り先として受け付ける値と破棄する値を検証する。
// 別のオリジンを指す値を通すとログインの導線がオープンリダイレクトになる。
func TestSanitizeReturnTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "同一オリジンの相対パス", raw: "/settings/invitations", want: "/settings/invitations"},
		{name: "クエリは残す", raw: "/search?q=cutre", want: "/search?q=cutre"},
		{name: "フラグメントは落とす", raw: "/help#section", want: "/help"},
		{name: "空の値", raw: "", want: ""},
		{name: "相対パスでない", raw: "settings", want: ""},
		{name: "スキーム付きの絶対URL", raw: "https://example.com/", want: ""},
		{name: "スキーム相対のURL", raw: "//example.com/", want: ""},
		{name: "バックスラッシュで別オリジンに解釈される値", raw: `/\example.com`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := middleware.SanitizeReturnTo(tt.raw); got != tt.want {
				t.Errorf("SanitizeReturnTo(%q) = %q、期待値 = %q", tt.raw, got, tt.want)
			}
		})
	}
}
