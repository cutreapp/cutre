// Package auth はパスワードのハッシュ化など、認証のための技術ユーティリティを提供する。
//
// 標準ライブラリと外部の暗号ライブラリだけに依存し、Cutreの他のパッケージには依存しない。
// 内部依存を持たないことで、どの層からも循環importの心配なく呼び出せる。
package auth

import (
	"errors"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost はパスワードのハッシュ化に使うコスト。
// テストは TestBcryptCost へ下げてハッシュ化を高速化する。
var BcryptCost = bcrypt.DefaultCost

// TestBcryptCost はテストで使うbcryptの最小コスト。
const TestBcryptCost = bcrypt.MinCost

const (
	// MinPasswordLength は最小のパスワード長。
	// エンコーディングによらず「8文字」と読めるようrune単位で数える。
	MinPasswordLength = 8

	// MaxPasswordLength はバイト単位の最大のパスワード長。
	// bcryptが扱える入力は72バイトまでであり、使用するライブラリも超過時にエラーを返すため、
	// 入力検証で同じ上限を明示する。
	MaxPasswordLength = 72
)

// ValidatePasswordStrength が返すsentinel error。
// authをi18nに依存させないため翻訳済みのメッセージではなくsentinelとし、
// 呼び出し側 (validator) が errors.Is で判別して翻訳を解決する。
var (
	ErrPasswordTooShort = errors.New("password is too short")
	ErrPasswordTooLong  = errors.New("password is too long")
)

// HashPassword は平文パスワードを BcryptCost でハッシュ化する。
// bcryptはハッシュごとのソルトを生成して埋め込むため、同じパスワードでも毎回異なる結果になる。
//
// 渡す平文は ValidatePasswordStrength を通したものに限る。
// 72バイトを超える入力にはbcryptがエラーを返し、入力の誤りをバリデーションエラーとして扱えなくなるため。
func HashPassword(plainPassword string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), BcryptCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// CheckPassword はbcryptハッシュが平文パスワードと一致するかを返す。
// 一致すればnilを、しなければ非nilのエラーを返す。
func CheckPassword(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

// dummyPasswordDigests はコストごとに1度だけ作る、照合の時間を揃えるためのハッシュ。
var dummyPasswordDigests sync.Map

// CheckPasswordWithoutDigest は照合するハッシュが無いときに、CheckPasswordと同じだけ時間を掛ける。
//
// ログインで宛先のアカウントが無いときに照合を省くと、応答がbcryptの分だけ速くなり、
// その差からメールアドレスが登録済みかどうかを推測できてしまう。
// 使い捨てのハッシュと照合して、アカウントが無いときも有るときと同じ計算をする。
// 結果は常に不一致のため返さない。
func CheckPasswordWithoutDigest(plainPassword string) {
	// テストがコストを下げるため、ハッシュは今のコストごとに作る。
	cost := BcryptCost
	digest, ok := dummyPasswordDigests.Load(cost)
	if !ok {
		// 平文は照合の相手として使うだけで、秘密ではない。
		hash, err := bcrypt.GenerateFromPassword([]byte("cutre-dummy-password"), cost)
		if err != nil {
			return
		}
		digest, _ = dummyPasswordDigests.LoadOrStore(cost, hash)
	}

	_ = bcrypt.CompareHashAndPassword(digest.([]byte), []byte(plainPassword))
}

// ValidatePasswordStrength はパスワードが長さのポリシーを満たすかを検証する。
//
// 文字種は制限しない。
// 日本語などの非ASCIIのパスワードを許すため最小はrune単位で数え、
// 最大はbcryptの72バイトの入力上限に合わせてバイト単位で数える。
// 空のパスワードは短すぎるものとして扱うため、「入力してください」を別に出したい呼び出し側は
// この関数を呼ぶ前に空かどうかを確認する。
func ValidatePasswordStrength(plainPassword string) error {
	if utf8.RuneCountInString(plainPassword) < MinPasswordLength {
		return ErrPasswordTooShort
	}

	if len(plainPassword) > MaxPasswordLength {
		return ErrPasswordTooLong
	}

	return nil
}
