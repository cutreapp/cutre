package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// secureTokenBytes はセキュアトークンの元にする乱数のバイト数。
// 32バイト (256ビット) はbase64urlでパディングの無い43文字になり、
// 招待リンクのようにURLへ載せても短く収まる。
const secureTokenBytes = 32

// GenerateSecureToken は暗号論的乱数のURLセーフなトークンを返す。
// セッショントークンのように、推測できないことだけを求められる不透明な値に使う。
//
// base64urlでエンコードするのは、Cookieの値やURLのパスにそのまま置けるようにするため。
// パディングを落とすのは、`=` がCookieの値の区切りと紛らわしいためである。
func GenerateSecureToken() (string, error) {
	b := make([]byte, secureTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken はトークンの16進表記のSHA-256ダイジェストを返す。
// トークンを平文ではなくダイジェストで保存し、完全一致で引くために使う。
//
// ここでbcryptではなくSHA-256を使うのは意図的である。
// GenerateSecureToken が作るトークンは高エントロピーの乱数で、人が決める低エントロピーの秘密とは違い、
// 総当たりを遅くするためのソルトや低速ハッシュを必要としない。
// 決定的なダイジェストであることが、保存した値を索引付きの完全一致で引ける条件でもある。
func HashToken(token string) string {
	digest := sha256.Sum256([]byte(token))

	return hex.EncodeToString(digest[:])
}
