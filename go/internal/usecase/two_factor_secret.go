package usecase

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
)

// twoFactorSecretAssociatedData はTOTPの秘密鍵の暗号文を持ち主に結び付ける付加データ (ユーザーIDの16バイト) を返す。
// 暗号化と復号で同じ値を渡さないと復号できないため、どのUseCaseもこの関数で作る。
func twoFactorSecretAssociatedData(userID model.UserID) []byte {
	id := uuid.UUID(userID)

	return id[:]
}

// decryptTwoFactorSecret はユーザーの二要素認証の設定から、平文のTOTPの秘密鍵を取り出す。
func decryptTwoFactorSecret(key *auth.TwoFactorKey, twoFactorAuth *model.UserTwoFactorAuth) (string, error) {
	secret, err := key.DecryptTOTPSecret(twoFactorAuth.SecretCiphertext, twoFactorSecretAssociatedData(twoFactorAuth.UserID))
	if err != nil {
		return "", fmt.Errorf("TOTPの秘密鍵の復号に失敗: %w", err)
	}

	return secret, nil
}

// TwoFactorAuthSetup は、認証アプリへの登録の途中で画面に出す秘密鍵。
type TwoFactorAuthSetup struct {
	// Secret はbase32の秘密鍵。認証アプリへ手で入力してもらう。
	Secret string
	// OTPAuthURL は秘密鍵を認証アプリへ登録するotpauth URI。QRコードとリンクにする。
	OTPAuthURL string
}

// newTwoFactorAuthSetup は秘密鍵から、画面に出す TwoFactorAuthSetup を作る。
// 認証アプリにはアットネームを表示し、どのアカウントのコードかを見分けられるようにする。
func newTwoFactorAuthSetup(secret string, user *model.User) (*TwoFactorAuthSetup, error) {
	url, err := auth.BuildOTPAuthURL(secret, user.Atname)
	if err != nil {
		return nil, fmt.Errorf("otpauth URIの作成に失敗: %w", err)
	}

	return &TwoFactorAuthSetup{Secret: secret, OTPAuthURL: url}, nil
}
