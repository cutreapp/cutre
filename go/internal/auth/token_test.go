package auth_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
)

// TestGenerateSecureToken は、生成したトークンがURLへそのまま置ける形式で、
// 呼び出しごとに異なる値になることを検証する。
func TestGenerateSecureToken(t *testing.T) {
	t.Parallel()

	token, err := auth.GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken()のエラー = %v", err)
	}

	// 32バイトの乱数をパディング無しのbase64urlにすると43文字になる。
	if len(token) != 43 {
		t.Errorf("トークンの長さ = %d、期待値 = 43", len(token))
	}

	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("トークンのデコードのエラー = %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("トークンの元のバイト数 = %d、期待値 = 32", len(decoded))
	}

	other, err := auth.GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken()のエラー = %v", err)
	}
	if token == other {
		t.Error("2回の呼び出しが同じトークンを返した")
	}
}

// TestHashToken は、同じトークンが常に同じダイジェストになり、
// 異なるトークンが異なるダイジェストになることを検証する。
// 保存したダイジェストを完全一致で引けることが、この性質に依存している。
func TestHashToken(t *testing.T) {
	t.Parallel()

	digest := auth.HashToken("token")

	if got := auth.HashToken("token"); got != digest {
		t.Errorf("同じトークンのダイジェスト = %q、期待値 = %q", got, digest)
	}
	if got := auth.HashToken("other"); got == digest {
		t.Error("異なるトークンが同じダイジェストになった")
	}
	if digest == "token" {
		t.Error("ダイジェストが平文のトークンと同じ値になっている")
	}

	// SHA-256の16進表記は64文字になる。
	if len(digest) != 64 {
		t.Errorf("ダイジェストの長さ = %d、期待値 = 64", len(digest))
	}
	if _, err := hex.DecodeString(digest); err != nil {
		t.Errorf("ダイジェストが16進表記ではない: %v", err)
	}
}
