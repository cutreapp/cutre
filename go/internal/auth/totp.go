package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"strings"
	"time"
	"unicode"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/text/width"
)

const (
	// totpIssuer は認証アプリに表示し、otpauth URIに埋め込むissuer。
	totpIssuer = "Cutre"

	// totpPeriod はTOTPのタイムステップの秒数。認証アプリの既定値の30秒にする。
	totpPeriod = 30

	// totpSkew は照合で許す前後のタイムステップの数。
	// 端末の時計の前後30秒までのずれを許し、受け付ける幅はそれ以上広げない。
	totpSkew = 1

	// totpSecretBytes は生成するTOTPの秘密鍵のバイト数。
	// 20バイト (160ビット) はRFC 4226が推奨する長さで、base32で32文字になる。
	totpSecretBytes = 20

	// TOTPCodeLength はTOTPのコードの桁数。
	TOTPCodeLength = 6
)

// totpOpts はコードの生成と照合で揃えるパラメーター。認証アプリの既定値 (SHA1・6桁・30秒) にする。
var totpOpts = totp.ValidateOpts{
	Period:    totpPeriod,
	Digits:    otp.DigitsSix,
	Algorithm: otp.AlgorithmSHA1,
}

// totpSecretEncoding はpquerna/otpが秘密鍵に使うbase32 (標準のアルファベット・パディング無し)。
var totpSecretEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret は暗号論的乱数のTOTPの秘密鍵をbase32で返す。
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, totpSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return totpSecretEncoding.EncodeToString(b), nil
}

// BuildOTPAuthURL はbase32の秘密鍵を認証アプリへ登録するotpauth URIを返す。
// accountName は認証アプリに表示するアカウントの名前 (アットネームなど)。
func BuildOTPAuthURL(secret, accountName string) (string, error) {
	raw, err := totpSecretEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountName,
		Period:      totpOpts.Period,
		Digits:      totpOpts.Digits,
		Algorithm:   totpOpts.Algorithm,
		Secret:      raw,
	})
	if err != nil {
		return "", err
	}

	return key.URL(), nil
}

// TOTPStep はtの時点のタイムステップ (Unix時刻をタイムステップの秒数で割った値) を返す。
func TOTPStep(t time.Time) int64 {
	return t.Unix() / totpPeriod
}

// MatchTOTPCode は、正規化したコードがnowの前後 totpSkew ステップのいずれかで秘密鍵に一致するかを照合し、
// 一致したタイムステップを返す。一致しないときはokをfalseにする。
//
// 真偽値だけでなくステップを返すのは、呼び出し側が最後に使ったステップと比べ、
// 一度受け付けたコードを使い回されたときに拒否できるようにするため。
// 不正な入力 (6桁の数字でないコード・読めない秘密鍵) もエラーにせず、不一致として扱う。
func MatchTOTPCode(secret, normalizedCode string, now time.Time) (step int64, ok bool) {
	if !isTOTPCodeShape(normalizedCode) {
		return 0, false
	}

	current := TOTPStep(now)
	// 新しいステップから順に照合し、まれに複数のステップで一致したときも新しい方を返す。
	for s := current + totpSkew; s >= current-totpSkew; s-- {
		code, err := totp.GenerateCodeCustom(secret, time.Unix(s*totpPeriod, 0), totpOpts)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(normalizedCode)) == 1 {
			return s, true
		}
	}

	return 0, false
}

// NormalizeTOTPCode は入力されたTOTPのコードから、貼り付けで混じった空白とハイフンを取り除く。
// 全角の数字は半角にする。日本語入力のまま打った数字も受け付けるため。
func NormalizeTOTPCode(input string) string {
	return stripSeparators(width.Narrow.String(input))
}

// isTOTPCodeShape はコードが TOTPCodeLength 桁の数字かを返す。
func isTOTPCodeShape(code string) bool {
	if len(code) != TOTPCodeLength {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// separatorRunes は空白のほかに区切りとして取り除く文字。
// 日本語入力のまま打ったハイフンは長音符 (U+30FC) になり、width.Narrow を通すと半角の長音符 (U+FF70) になる。
// 貼り付けたコードにも似たハイフン類 (U+2010・U+2011・U+2013・U+2014・U+2212) が混じるため、あわせて取り除く。
const separatorRunes = "-ーｰ‐‑–—−"

// stripSeparators は空白 (全角の空白を含む) とハイフン類を取り除く。
func stripSeparators(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune(separatorRunes, r) {
			return -1
		}
		return r
	}, s)
}
