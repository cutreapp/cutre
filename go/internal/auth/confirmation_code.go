package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// confirmationCodeMax は確認コードの取りうる値の数 (000000〜999999)。
const confirmationCodeMax = 1_000_000

// GenerateConfirmationCode は暗号論的乱数で6桁の数字の確認コードを返す。
// 先頭の0も桁として残し、メールで読んで入力しやすい固定の長さにする。
func GenerateConfirmationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(confirmationCodeMax))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}
