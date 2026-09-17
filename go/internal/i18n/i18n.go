// Package i18n は翻訳の取得、パスとロケールの相互変換、ヘッダーからの言語判定、contextが運ぶリクエストのロケールを提供する。
//
// ミドルウェアから翻訳を利用できるよう、本パッケージは internal/middleware への依存を持たない。
package i18n

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// 翻訳ファイルをバイナリに埋め込み、実行時に外部ファイルを必要としないようにする。
//
//go:embed locales/*.toml
var localesFS embed.FS

// サポートするロケール。
const (
	LangJa      = "ja"
	LangEn      = "en"
	DefaultLang = LangJa
)

// supportedLangs は翻訳を持つロケールを、bundleが読み込む順に並べたもの。
// 読み込む対象と対応判定が同じ一覧を見ることで、ロケールを増やしたときに
// bundleに無いロケールをSetLocaleが受け入れてしまう食い違いを防ぐ。
var supportedLangs = []string{LangJa, LangEn}

// contextキーの型。他パッケージのキーとの衝突を避けるため非公開の型にする。
type contextKey string

const (
	localeContextKey          contextKey = "locale"
	localizerContextKey       contextKey = "localizer"
	suggestedLocaleContextKey contextKey = "suggested_locale"
)

// bundle は全ロケールの翻訳を保持する。起動時に一度だけ構築し、以降は読み取り専用で扱う。
var bundle *goi18n.Bundle

func init() {
	bundle = goi18n.NewBundle(language.Japanese)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, lang := range supportedLangs {
		name := fmt.Sprintf("locales/%s.toml", lang)

		data, err := localesFS.ReadFile(name)
		if err != nil {
			// 翻訳ファイルはビルド時に埋め込まれるため、読み込みの失敗はファイル名とロケールの
			// 不整合を意味する。そのロケールの翻訳が欠けたまま黙って起動せず、ここで落とす。
			panic(fmt.Sprintf("翻訳ファイルの読み込みに失敗しました: %s: %v", name, err))
		}

		bundle.MustParseMessageFileBytes(data, fmt.Sprintf("%s.toml", lang))
	}
}

// T はctxのロケールでmessageIDを翻訳する。
// 翻訳が見つからない場合はmessageIDをそのまま返すため、キーの誤りは描画結果に現れる。
func T(ctx context.Context, messageID string, templateData ...map[string]any) string {
	config := &goi18n.LocalizeConfig{MessageID: messageID}
	if len(templateData) > 0 && templateData[0] != nil {
		config.TemplateData = templateData[0]
	}

	message, err := localizer(ctx).Localize(config)
	if err != nil {
		return messageID
	}

	return message
}

// GetLocale はctxのロケールを返す。未設定の場合はDefaultLangを返す。
func GetLocale(ctx context.Context) string {
	if locale, ok := ctx.Value(localeContextKey).(string); ok {
		return locale
	}

	return DefaultLang
}

// SetLocale はロケールと、そのロケールのLocalizerを載せたctxのコピーを返す。
// Localizerを併せて持たせるのは、1リクエストが描画する翻訳の解決をTの呼び出しごとではなく1回で済ませるため。
//
// 対応しないロケールはDefaultLangに正規化する。go-i18nのLocalizerは未知の言語を言語距離で解決するため
// (例: "fr" は英語に解決される)、正規化しないと `lang` 属性と本文の言語が食い違う。
func SetLocale(ctx context.Context, locale string) context.Context {
	if !IsSupportedLang(locale) {
		locale = DefaultLang
	}

	ctx = context.WithValue(ctx, localeContextKey, locale)

	return context.WithValue(ctx, localizerContextKey, goi18n.NewLocalizer(bundle, locale))
}

// localizer はctxのLocalizerを返す。無い場合はctxのロケールから作るため、
// SetLocaleを経ていないctx (ミドルウェアを通らないテストなど) でもTが機能する。
func localizer(ctx context.Context) *goi18n.Localizer {
	if l, ok := ctx.Value(localizerContextKey).(*goi18n.Localizer); ok {
		return l
	}

	return goi18n.NewLocalizer(bundle, GetLocale(ctx))
}

// DetectLanguage はリクエストのAccept-Languageヘッダーが求める言語のうち、本アプリケーションが
// 翻訳を持つものを品質値の高い順に選んで返す。該当するものが無ければ空文字列を返す。
// クライアントが明示した言語だけを採用するため、ParseAcceptLanguageで並べたタグからRawで言語を取り出す。
//
// 該当が無いときに既定ロケールへ寄せないのは、本関数の結果を使うのが別の言語版の案内を出すかどうかの
// 判定だけのため。表示する言語は開いたURLで決まる。ここで既定ロケールへ寄せると、日本語を求めていない
// 利用者にも日本語版の案内を出すことになる。
func DetectLanguage(r *http.Request) string {
	tags, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))

	for _, tag := range tags {
		base, _, _ := tag.Raw()
		if IsSupportedLang(base.String()) {
			return base.String()
		}
	}

	return ""
}

// SetSuggestedLocale は別の言語版として案内すべきロケールを載せたctxのコピーを返す。
// 案内しない場合は空文字列を渡す。翻訳を持たないロケールは案内しようがないため空文字列に正規化する。
func SetSuggestedLocale(ctx context.Context, locale string) context.Context {
	if !IsSupportedLang(locale) {
		locale = ""
	}

	return context.WithValue(ctx, suggestedLocaleContextKey, locale)
}

// GetSuggestedLocale はctxの案内すべきロケールを返す。案内しない場合は空文字列を返す。
func GetSuggestedLocale(ctx context.Context) string {
	if locale, ok := ctx.Value(suggestedLocaleContextKey).(string); ok {
		return locale
	}

	return ""
}

// IsSupportedLang はlocaleが本アプリケーションの翻訳を持つロケールかどうかを返す。
func IsSupportedLang(locale string) bool {
	return slices.Contains(supportedLangs, locale)
}

// SupportedLangs は翻訳を持つロケールを返す。
// 呼び出し元が返り値を書き換えても内部の一覧に及ばないよう複製を返す。
func SupportedLangs() []string {
	return slices.Clone(supportedLangs)
}

// LocaleFromPath はリクエストのパスが属する言語版のロケールを返す。
// 既定ロケールは言語コードを持たないパスに置くため、どの言語コードにも該当しないパスはDefaultLangになる。
//
// 言語コードはパスセグメントとして照合する。前方一致だけで判定すると "/english" のような
// 無関係なパスが言語版として扱われる。
func LocaleFromPath(path string) string {
	for _, lang := range supportedLangs {
		if lang == DefaultLang {
			continue
		}

		if path == "/"+lang || strings.HasPrefix(path, "/"+lang+"/") {
			return lang
		}
	}

	return DefaultLang
}

// LocalePath はロケールに依存しないパスを、localeの言語版のパスへ変換する。
// pathには言語コードを含まない形 ("/" や "/items") を渡す。
//
// 既定ロケールのパスに言語コードを付けないのは、同じ内容を言語コードあり / なしの
// 両方のURLで返さないため。対応しないロケールは既定ロケールのパスにする。
func LocalePath(locale, path string) string {
	if !IsSupportedLang(locale) || locale == DefaultLang {
		return path
	}

	// 言語版のトップは "/en/" ではなく "/en" を正規の形にする。
	// 末尾スラッシュはリポジトリ全体で落とす方針のため、ここで付けると正規化のリダイレクトを踏むURLになる。
	if path == "/" {
		return "/" + locale
	}

	return "/" + locale + path
}
