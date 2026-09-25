package model

// Locale はアカウントに保存する表示言語。
//
// 値域はアプリが翻訳を持つ言語に閉じる。
// internal/i18n ではなくDomain層に置くのは、どの言語で書くかがユーザーの属性であり、
// Domain層はPresentation層の internal/i18n に依存できないため。
type Locale string

// アプリが翻訳を持つ表示言語。
const (
	LocaleJa Locale = "ja"
	LocaleEn Locale = "en"
)

// DefaultLocale はロケールが決まっていない場面で使うロケール。
// Cutreの利用者は日本語話者を想定しているため日本語とする。
const DefaultLocale = LocaleJa

// Locales はすべての表示言語を返す。
// 呼び出しごとに新しいスライスを返すため、呼び出し側の変更が他へ波及しない。
func Locales() []Locale {
	return []Locale{LocaleJa, LocaleEn}
}

// ParseLocale はsが表す Locale を、表しているかどうかとともに返す。
// リクエストのパスやジョブ引数など、アプリの外から来た値が型に入る境界で使う。
func ParseLocale(s string) (Locale, bool) {
	for _, locale := range Locales() {
		if string(locale) == s {
			return locale, true
		}
	}

	return "", false
}
