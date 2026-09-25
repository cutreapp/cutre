package auth_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
)

// TestMain はハッシュ化のコストを下げてからテストを実行する。
// コストはパッケージレベルの変数のため、並行するテストが読み書きで競合しないよう
// テストを開始する前に1度だけ書き換える。
func TestMain(m *testing.M) {
	auth.BcryptCost = auth.TestBcryptCost

	os.Exit(m.Run())
}

// TestHashPassword_Salted は、同じパスワードでも毎回異なるハッシュになり、
// どちらのハッシュも元のパスワードと照合できることを検証する。
func TestHashPassword_Salted(t *testing.T) {
	t.Parallel()

	const password = "password123"

	first, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword()のエラー = %v", err)
	}

	second, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword()のエラー = %v", err)
	}

	if first == second {
		t.Error("同じパスワードから同じハッシュが得られた、ソルトにより異なることを期待")
	}

	if strings.Contains(first, password) {
		t.Error("ハッシュが平文のパスワードを含んでいる")
	}

	for _, digest := range []string{first, second} {
		if err := auth.CheckPassword(digest, password); err != nil {
			t.Errorf("CheckPassword()のエラー = %v", err)
		}
	}
}

// TestCheckPassword_Mismatch は、違うパスワードとの照合が失敗することを検証する。
func TestCheckPassword_Mismatch(t *testing.T) {
	t.Parallel()

	digest, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword()のエラー = %v", err)
	}

	if err := auth.CheckPassword(digest, "password124"); err == nil {
		t.Error("エラーを期待したが、nilだった")
	}
}

// TestCheckPasswordWithoutDigest は、照合の上限を超える長さを含むどの入力でも止まらずに返ることを検証する。
// 掛かる時間はテストの環境に左右されるため比べない。
func TestCheckPasswordWithoutDigest(t *testing.T) {
	t.Parallel()

	for _, password := range []string{"", "password123", strings.Repeat("a", auth.MaxPasswordLength+1)} {
		auth.CheckPasswordWithoutDigest(password)
	}
}

// TestValidatePasswordStrength は、長さのポリシーがrune単位の下限と
// バイト単位の上限で判定されることを検証する。
func TestValidatePasswordStrength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		want     error
	}{
		{name: "8文字ちょうどは通る", password: strings.Repeat("a", 8), want: nil},
		{name: "7文字は短すぎる", password: strings.Repeat("a", 7), want: auth.ErrPasswordTooShort},
		{name: "空のパスワードは短すぎる", password: "", want: auth.ErrPasswordTooShort},
		// 日本語は1文字3バイトのため、8文字 (24バイト) は下限を満たし上限にも収まる。
		{name: "日本語8文字は通る", password: strings.Repeat("あ", 8), want: nil},
		// 日本語7文字 (21バイト) は上限に収まるが、文字数では下限に届かない。
		{name: "日本語7文字は短すぎる", password: strings.Repeat("あ", 7), want: auth.ErrPasswordTooShort},
		{name: "72バイトちょうどは通る", password: strings.Repeat("a", 72), want: nil},
		{name: "73バイトは長すぎる", password: strings.Repeat("a", 73), want: auth.ErrPasswordTooLong},
		// 日本語25文字は75バイトで、文字数では上限に見えなくてもバイト数で超える。
		{name: "日本語25文字は長すぎる", password: strings.Repeat("あ", 25), want: auth.ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := auth.ValidatePasswordStrength(tt.password)
			if !errors.Is(err, tt.want) {
				t.Errorf("ValidatePasswordStrength()のエラー = %v、期待値 = %v", err, tt.want)
			}
		})
	}
}
