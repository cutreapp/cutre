package auth_test

import (
	"regexp"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
)

// TestGenerateConfirmationCode は、確認コードが常に6桁の数字になることを検証する。
func TestGenerateConfirmationCode(t *testing.T) {
	t.Parallel()

	sixDigits := regexp.MustCompile(`^[0-9]{6}$`)
	for range 100 {
		code, err := auth.GenerateConfirmationCode()
		if err != nil {
			t.Fatalf("GenerateConfirmationCode()のエラー = %v", err)
		}
		if !sixDigits.MatchString(code) {
			t.Fatalf("確認コード = %q、6桁の数字を期待", code)
		}
	}
}
