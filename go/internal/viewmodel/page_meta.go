// Package viewmodel はテンプレートが描画する形へデータを整えるプレゼンテーション層のパッケージ。
package viewmodel

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/i18n"
)

// SiteName はサイトの名前。ロケールによらない固有名詞のため翻訳ファイルには置かない。
const SiteName = "Cutre"

// Alternate は同じ内容を別の言語で提供するページを指す。
// hreflangの出力と言語スイッチャーの両方がこれを読む。
type Alternate struct {
	// Lang はその言語版のBCP 47言語タグ。hreflang属性に出す。
	Lang string

	// URL はその言語版の絶対URL。指す先のcanonicalと一致させる。
	URL string

	// Endonym はその言語自身による言語名 ("日本語" / "English")。言語スイッチャーの表記に使う。
	// 英語に統一した表記にすると、その言語しか読めない利用者が自分の言語を見つけられない。
	Endonym string

	// Current は表示中の言語版かどうか。言語スイッチャーが現在の選択として示す。
	Current bool
}

// LocaleSuggestion は表示中の言語版とは別の言語版があることを知らせる案内。
// 文言は案内先の言語で持つ。案内を読む相手はその言語を求めている利用者であり、
// 表示中の言語で書くと当人が案内に気付けない。
type LocaleSuggestion struct {
	// Lang は案内先の言語版のBCP 47言語タグ。文言の言語を示すlang属性に出す。
	Lang string

	// URL は案内先の言語版の絶対URL。
	URL string

	// Label は案内の領域名。案内はランドマークとして描画され、支援技術の領域一覧にこの名前で並ぶ。
	Label string

	// Message は別の言語版があることを伝える本文。
	Message string

	// SwitchLabel は案内先の言語版へ移動するリンクの文言。
	SwitchLabel string

	// DismissLabel は案内を閉じるボタンの文言。
	DismissLabel string

	// DismissLocale は案内を閉じたときに利用者の選択として記録するロケール。
	// 閉じる操作は「表示中の言語版のままでよい」という選択のため、表示中のロケールが入る。
	DismissLocale string
}

// siteSuffix はページ固有のタイトルの後ろに付けるサイト名。
const siteSuffix = " | " + SiteName

// PageMeta は共通レイアウトが描画するページ単位のメタ情報を保持する。
type PageMeta struct {
	// Title は<title>とog:titleに出すページのタイトル。
	Title string

	// Description はmeta descriptionとog:descriptionに出すページの要約。
	Description string

	// CanonicalURL はページが自身の正規のアドレスとして宣言する絶対URL。
	// 空のときはcanonicalとog:urlを出力しない。空の値は「canonicalが無い」ことにはならず、
	// リクエストされたURL自身を指す宣言になるため、クエリ違いのURLがそれぞれ自分を正規のアドレスとして名乗ってしまう。
	CanonicalURL string

	// Alternates はこのページの全言語版 (自分自身を含む) を指すhreflangの一覧。
	// 自己参照を含む相互参照の形でないと検索エンジンがアノテーションを無視するため、
	// 表示中のロケールの分も落とさずに持つ。
	Alternates []Alternate

	// AssetVersion はCSS / JSのURLに付けるキャッシュ無効化用のクエリ値。
	AssetVersion string

	// OGLocale はog:localeに出す、ページの言語と地域の組み合わせ ("ja_JP" / "en_US")。
	// Open Graphは言語だけの表記を受け付けないため、lang属性のロケールから変換して持つ。
	OGLocale string

	// LocaleSuggestion は別の言語版を知らせる案内。案内するものが無いときはnil。
	LocaleSuggestion *LocaleSuggestion

	// PreconnectOrigins はこのページが接続する第三者オリジン。
	// 使うページだけで宣言し、DNS・TCP・TLSの接続準備を先行させる。
	PreconnectOrigins []string

	// NoIndex は検索エンジンにこのページをインデックスさせないかどうか。
	// 真のときはcanonicalを宣言しない。「正規はこのアドレス」と「インデックスするな」は矛盾したシグナルになる。
	NoIndex bool
}

// ogLocaleFromLocale はロケールをOpen Graphが要求する言語_地域の表記へ変換する。
// 対応表に無いロケールは既定のロケールの表記にし、og:localeが空になる経路を作らない。
func ogLocaleFromLocale(locale string) string {
	switch locale {
	case i18n.LangEn:
		return "en_US"
	default:
		return "ja_JP"
	}
}

// DefaultPageMeta は全ページの基準となるメタ情報を返す。
// TitleとDescriptionはサイト全体の既定値で、呼び出し元がページ固有の文言で上書きする。
//
// pathには言語コードを含まない、そのページを代表する形 ("/" や "/items") を渡す。
// canonicalは表示中のロケールの言語版を、Alternatesは全言語版を、いずれもこのpathから組み立てる。
// canonicalが自分自身を指すのは、言語版どうしが重複コンテンツではなく別ページのため。
// 片方をもう片方のcanonicalにすると、正規化された側が検索結果から落ちる。
func DefaultPageMeta(ctx context.Context, cfg *config.Config, path string) PageMeta {
	langs := i18n.SupportedLangs()
	currentLocale := i18n.GetLocale(ctx)
	suggestedLocale := i18n.GetSuggestedLocale(ctx)

	alternates := make([]Alternate, 0, len(langs))

	var suggestion *LocaleSuggestion

	for _, lang := range langs {
		url := cfg.AppURL() + i18n.LocalePath(lang, path)

		alternates = append(alternates, Alternate{
			Lang:    lang,
			URL:     url,
			Endonym: i18n.T(i18n.SetLocale(ctx, lang), "language_endonym"),
			Current: lang == currentLocale,
		})

		if lang == suggestedLocale {
			suggestion = newLocaleSuggestion(ctx, lang, url)
		}
	}

	return PageMeta{
		Title:        i18n.T(ctx, "default_title"),
		Description:  i18n.T(ctx, "default_description"),
		CanonicalURL: cfg.AppURL() + i18n.LocalePath(currentLocale, path),
		AssetVersion: cfg.AssetVersion(),
		OGLocale:     ogLocaleFromLocale(currentLocale),
		Alternates:   alternates,

		LocaleSuggestion: suggestion,
	}
}

// newLocaleSuggestion は案内先の言語で書かれた案内を組み立てる。
//
// 翻訳の解決にはロケールを差し替えたctxを使う。templはctxのロケールを差し替える手段を持たないため、
// 表示中の言語とは別の言語で文言を引く変換はここで行う。
func newLocaleSuggestion(ctx context.Context, locale, url string) *LocaleSuggestion {
	suggestedCtx := i18n.SetLocale(ctx, locale)

	return &LocaleSuggestion{
		Lang:          locale,
		URL:           url,
		Label:         i18n.T(suggestedCtx, "locale_suggestion_label"),
		Message:       i18n.T(suggestedCtx, "locale_suggestion_message"),
		SwitchLabel:   i18n.T(suggestedCtx, "locale_suggestion_switch"),
		DismissLabel:  i18n.T(suggestedCtx, "locale_suggestion_dismiss"),
		DismissLocale: i18n.GetLocale(ctx),
	}
}

// SetTitle はサイト名のサフィックス付きでタイトルを設定する。
// ブラウザのタブ・ブックマーク・検索結果はいずれも末尾を切り詰めるため、ページ同士を見分けさせる部分を先に置く。
func (p *PageMeta) SetTitle(ctx context.Context, titleKey string) {
	p.Title = i18n.T(ctx, titleKey) + siteSuffix
}

// SetTitleWithoutSuffix はサフィックス無しでタイトルを設定する。
// タイトル自体がサイト名になるトップページで使う。
func (p *PageMeta) SetTitleWithoutSuffix(ctx context.Context, titleKey string) {
	p.Title = i18n.T(ctx, titleKey)
}

// ErrorPageMeta はエラーページの基準となるメタ情報を返す。
// TitleとDescriptionはサイト全体の既定値で、呼び出し元がそのエラーの文言で上書きする。
//
// CanonicalURL・Alternates・LocaleSuggestionはいずれも持たせない。
// エラー応答は通常のページ本文を返していないため、正規ページとして扱わず、
// 通常ページの別言語版への参照や案内も出さない。
func ErrorPageMeta(ctx context.Context, cfg *config.Config) PageMeta {
	return PageMeta{
		Title:        i18n.T(ctx, "default_title"),
		Description:  i18n.T(ctx, "default_description"),
		AssetVersion: cfg.AssetVersion(),
		OGLocale:     ogLocaleFromLocale(i18n.GetLocale(ctx)),
	}
}

// SignedInPageMeta はログイン後のページの基準となるメタ情報を返す。
//
// CanonicalURL・Alternates・LocaleSuggestionはいずれも持たせない。
// ログイン後のページは表示言語を users.locale で決め、言語版のURLを持たないため、
// 別言語版への参照も、別言語版があるという案内も成り立たない。
// 利用者ごとの内容で検索の対象にもならないため、正規のアドレスも宣言せず、インデックスも断る。
// 未ログインのクローラーはログイン画面へ送られて本文に届かないが、noindexはその前提が崩れたときの防御として付ける。
func SignedInPageMeta(ctx context.Context, cfg *config.Config) PageMeta {
	return PageMeta{
		Title:        i18n.T(ctx, "default_title"),
		Description:  i18n.T(ctx, "default_description"),
		AssetVersion: cfg.AssetVersion(),
		OGLocale:     ogLocaleFromLocale(i18n.GetLocale(ctx)),
		NoIndex:      true,
	}
}

// AddTurnstilePreconnect はウィジェットを描画するページの接続準備を宣言する。
// サイトキーが空なら外部通信しないため、接続準備も行わない。
func (p *PageMeta) AddTurnstilePreconnect(siteKey string) {
	if siteKey != "" {
		p.PreconnectOrigins = append(p.PreconnectOrigins, "https://challenges.cloudflare.com")
	}
}
