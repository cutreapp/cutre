package i18n

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func TestT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		locale    string
		messageID string
		want      string
	}{
		{
			name:      "日本語のスキップリンク",
			locale:    LangJa,
			messageID: "skip_to_main_content",
			want:      "メインコンテンツへスキップ",
		},
		{
			name:      "英語のスキップリンク",
			locale:    LangEn,
			messageID: "skip_to_main_content",
			want:      "Skip to main content",
		},
		{
			name:      "未対応のロケールではデフォルトロケールの翻訳を返す",
			locale:    "fr",
			messageID: "skip_to_main_content",
			want:      "メインコンテンツへスキップ",
		},
		{
			name:      "未定義のキーではキーをそのまま返す",
			locale:    LangJa,
			messageID: "undefined_message_id",
			want:      "undefined_message_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := T(SetLocale(context.Background(), tt.locale), tt.messageID)
			if got != tt.want {
				t.Errorf("T(ctx, %q) = %q、期待値 = %q", tt.messageID, got, tt.want)
			}
		})
	}
}

func TestT_DefaultLocale(t *testing.T) {
	t.Parallel()

	// ロケールを載せていないctxでも、デフォルトロケール (日本語) で翻訳できる。
	got := T(context.Background(), "skip_to_main_content")
	want := "メインコンテンツへスキップ"
	if got != want {
		t.Errorf("T(ctx, %q) = %q、期待値 = %q", "skip_to_main_content", got, want)
	}
}

func TestT_TemplateData(t *testing.T) {
	t.Parallel()

	// 共有bundleにテスト用の翻訳を加えず、並列テストから独立させる。
	testBundle := goi18n.NewBundle(language.Japanese)
	if err := testBundle.AddMessages(language.Japanese,
		&goi18n.Message{ID: "greeting", Other: "こんにちは、{{.Name}}さん"},
		&goi18n.Message{ID: "plain", Other: "こんにちは"},
	); err != nil {
		t.Fatalf("テスト用翻訳の登録のエラー = %v", err)
	}

	tests := []struct {
		name         string
		messageID    string
		templateData map[string]any
		want         string
	}{
		{
			name:         "プレースホルダーを渡された値で展開する",
			messageID:    "greeting",
			templateData: map[string]any{"Name": "太郎"},
			want:         "こんにちは、太郎さん",
		},
		{
			name:         "nil引数でも通常の翻訳を返す",
			messageID:    "plain",
			templateData: nil,
			want:         "こんにちは",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.WithValue(context.Background(), localizerContextKey, goi18n.NewLocalizer(testBundle, LangJa))
			if got := T(ctx, tt.messageID, tt.templateData); got != tt.want {
				t.Errorf("T(ctx, %q, %v) = %q、期待値 = %q", tt.messageID, tt.templateData, got, tt.want)
			}
		})
	}
}

func TestGetLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "日本語を載せたctx",
			ctx:  SetLocale(context.Background(), LangJa),
			want: LangJa,
		},
		{
			name: "英語を載せたctx",
			ctx:  SetLocale(context.Background(), LangEn),
			want: LangEn,
		},
		{
			name: "ロケールを載せていないctxではデフォルトロケールを返す",
			ctx:  context.Background(),
			want: DefaultLang,
		},
		{
			name: "未対応のロケールはデフォルトロケールに正規化される",
			ctx:  SetLocale(context.Background(), "fr"),
			want: DefaultLang,
		},
		{
			name: "後から載せたロケールで上書きされる",
			ctx:  SetLocale(SetLocale(context.Background(), LangJa), LangEn),
			want: LangEn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := GetLocale(tt.ctx); got != tt.want {
				t.Errorf("GetLocale(ctx) = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestDetectLanguage はクライアントが明示した対応言語だけを検出することを検証する。
// 検出結果は別の言語版の案内を出すかどうかの判定に使うため、求められていない言語を
// 既定ロケールとして返すと、日本語を求めていない利用者にも日本語版の案内が出る。
func TestDetectLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		acceptLanguage string
		want           string
	}{
		{
			name:           "日本語のみ",
			acceptLanguage: "ja",
			want:           LangJa,
		},
		{
			name:           "地域付きのタグはベース言語で判定する",
			acceptLanguage: "en-US,en;q=0.9",
			want:           LangEn,
		},
		{
			name:           "品質値の高い言語を優先する",
			acceptLanguage: "en;q=0.9,ja;q=0.5",
			want:           LangEn,
		},
		{
			name:           "並び順ではなく品質値で優先度を判定する",
			acceptLanguage: "ja;q=0.5,en;q=0.9",
			want:           LangEn,
		},
		{
			name:           "未対応の言語は読み飛ばして次の候補を見る",
			acceptLanguage: "fr,en;q=0.9,ja;q=0.8",
			want:           LangEn,
		},
		{
			name:           "対応する言語が無ければ空文字列",
			acceptLanguage: "fr-FR,de;q=0.9",
			want:           "",
		},
		{
			name:           "言語未指定のタグは検出結果にしない",
			acceptLanguage: "und",
			want:           "",
		},
		{
			name:           "言語未指定のタグを飛ばして明示された日本語を選ぶ",
			acceptLanguage: "und,ja;q=0.9",
			want:           LangJa,
		},
		{
			name:           "地域から言語を推測せず明示された日本語を選ぶ",
			acceptLanguage: "und-US,ja;q=0.9",
			want:           LangJa,
		},
		{
			name:           "Accept-Languageヘッダーが無ければ空文字列",
			acceptLanguage: "",
			want:           "",
		},
		{
			name:           "解析できないヘッダーなら空文字列",
			acceptLanguage: "!!!",
			want:           "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}

			if got := DetectLanguage(req); got != tt.want {
				t.Errorf("DetectLanguage() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestLocaleMessageIDsMatch は2つのロケールファイルが同じメッセージIDを定義していることを固定する。
// 片方にしか無いメッセージIDはもう一方の言語へフォールバックせず、TがメッセージIDをそのまま返すため、
// 欠けている側のロケールでは内部の識別子が画面に出る。
func TestLocaleMessageIDsMatch(t *testing.T) {
	t.Parallel()

	ja := localeMessageIDs(t, LangJa)
	en := localeMessageIDs(t, LangEn)

	if missing := missingMessageIDs(ja, en); len(missing) > 0 {
		t.Errorf("%s.tomlに%s.tomlのメッセージIDがありません: %v", LangEn, LangJa, missing)
	}
	if missing := missingMessageIDs(en, ja); len(missing) > 0 {
		t.Errorf("%s.tomlに%s.tomlのメッセージIDがありません: %v", LangJa, LangEn, missing)
	}
}

// localeMessageIDs は指定したロケールファイルが定義するメッセージIDを返す。
// 各メッセージはトップレベルのTOMLテーブルであるため、トップレベルのキーがメッセージIDになる。
func localeMessageIDs(t *testing.T, lang string) map[string]struct{} {
	t.Helper()

	data, err := localesFS.ReadFile(fmt.Sprintf("locales/%s.toml", lang))
	if err != nil {
		t.Fatalf("%s.tomlの読み込みのエラー = %v", lang, err)
	}

	var messages map[string]any
	if err := toml.Unmarshal(data, &messages); err != nil {
		t.Fatalf("%s.tomlの解析のエラー = %v", lang, err)
	}

	ids := make(map[string]struct{}, len(messages))
	for id := range messages {
		ids[id] = struct{}{}
	}

	return ids
}

// missingMessageIDs はfromにあってinに無いメッセージIDを返す。
// 失敗時の一覧が実行ごとに同じ順序になるようソートする。
func missingMessageIDs(from, in map[string]struct{}) []string {
	missing := make([]string, 0)
	for id := range from {
		if _, ok := in[id]; !ok {
			missing = append(missing, id)
		}
	}
	slices.Sort(missing)

	return missing
}

func TestSupportedLangs(t *testing.T) {
	t.Parallel()

	want := []string{LangJa, LangEn}
	if got := SupportedLangs(); !slices.Equal(got, want) {
		t.Errorf("SupportedLangs() = %v、期待値 = %v", got, want)
	}

	// 返り値を書き換えても内部の一覧は変わらない。
	// 呼び出し元がhreflangの並びを組み替えるだけで、以降のリクエストのロケール判定が壊れてはならない。
	SupportedLangs()[0] = "fr"
	if got := SupportedLangs(); !slices.Equal(got, want) {
		t.Errorf("返り値を書き換えたあとの SupportedLangs() = %v、期待値 = %v", got, want)
	}
}

func TestLocaleFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "言語コードを持たないトップは既定ロケール",
			path: "/",
			want: DefaultLang,
		},
		{
			name: "言語コードを持たないページは既定ロケール",
			path: "/items",
			want: DefaultLang,
		},
		{
			name: "英語版のトップ",
			path: "/en",
			want: LangEn,
		},
		{
			name: "英語版の配下のページ",
			path: "/en/items",
			want: LangEn,
		},
		{
			name: "言語コードで始まるだけの別の語は言語版ではない",
			path: "/english",
			want: DefaultLang,
		},
		{
			name: "大文字の言語コードは言語版ではない",
			path: "/EN",
			want: DefaultLang,
		},
		{
			name: "既定ロケールの言語コードは言語版として持たない",
			path: "/ja",
			want: DefaultLang,
		},
		{
			name: "公開ページ以外も既定ロケールになる",
			path: "/health",
			want: DefaultLang,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := LocaleFromPath(tt.path); got != tt.want {
				t.Errorf("LocaleFromPath(%q) = %q、期待値 = %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestLocalePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale string
		path   string
		want   string
	}{
		{
			name:   "既定ロケールのトップは言語コードを持たない",
			locale: LangJa,
			path:   "/",
			want:   "/",
		},
		{
			name:   "既定ロケールのページは言語コードを持たない",
			locale: LangJa,
			path:   "/items",
			want:   "/items",
		},
		{
			name:   "英語版のトップは末尾スラッシュを持たない",
			locale: LangEn,
			path:   "/",
			want:   "/en",
		},
		{
			name:   "英語版のページは言語コードを前置する",
			locale: LangEn,
			path:   "/items",
			want:   "/en/items",
		},
		{
			name:   "未対応のロケールは既定ロケールのパスにする",
			locale: "fr",
			path:   "/items",
			want:   "/items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := LocalePath(tt.locale, tt.path); got != tt.want {
				t.Errorf("LocalePath(%q, %q) = %q、期待値 = %q", tt.locale, tt.path, got, tt.want)
			}
		})
	}
}

// TestLocalePath_RoundTrip はLocalePathが作ったパスをLocaleFromPathが元のロケールとして読み戻せることを固定する。
// 2つの変換がずれると、hreflangが指すURLを開いたときに別の言語版が描画される。
func TestLocalePath_RoundTrip(t *testing.T) {
	t.Parallel()

	for _, lang := range SupportedLangs() {
		for _, path := range []string{"/", "/items", "/items/1"} {
			if got := LocaleFromPath(LocalePath(lang, path)); got != lang {
				t.Errorf("LocaleFromPath(LocalePath(%q, %q)) = %q、期待値 = %q", lang, path, got, lang)
			}
		}
	}
}

// TestSuggestedLocale は案内すべきロケールがcontextを通じて運ばれ、
// 翻訳を持たないロケールは案内の対象にならないことを検証する。
func TestSuggestedLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{name: "対応するロケールはそのまま運ばれる", locale: LangEn, want: LangEn},
		{name: "空文字列は案内しないことを表す", locale: "", want: ""},
		{name: "翻訳を持たないロケールは案内しない", locale: "fr", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := SetSuggestedLocale(context.Background(), tt.locale)

			if got := GetSuggestedLocale(ctx); got != tt.want {
				t.Errorf("GetSuggestedLocale() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestGetSuggestedLocale_Unset はミドルウェアを通らないctxでは案内しない扱いになることを検証する。
func TestGetSuggestedLocale_Unset(t *testing.T) {
	t.Parallel()

	if got := GetSuggestedLocale(context.Background()); got != "" {
		t.Errorf("GetSuggestedLocale() = %q、期待値 = 空文字列", got)
	}
}
