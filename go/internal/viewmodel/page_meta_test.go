package viewmodel_test

import (
	"context"
	"slices"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestDefaultPageMeta は既定値がctxのロケールで解決され、アセットバージョンが設定から入り、
// og:localeがロケールに対応する言語_地域の表記になることを検証する。
func TestDefaultPageMeta(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Env: "prod", Domain: "cutre.example.com", GitRev: "abc1234"}

	tests := []struct {
		name            string
		locale          string
		wantDescription string
		wantOGLocale    string
	}{
		{
			name:            "日本語",
			locale:          i18n.LangJa,
			wantDescription: "〝かわいい〟を交換！キャラクターグッズの物々交換を簡単にするサービスです",
			wantOGLocale:    "ja_JP",
		},
		{
			name:            "英語",
			locale:          i18n.LangEn,
			wantDescription: "Cutre makes trading character goods simple.",
			wantOGLocale:    "en_US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			meta := viewmodel.DefaultPageMeta(ctx, cfg, "/")

			if meta.Title != viewmodel.SiteName {
				t.Errorf("Title = %q、期待値 = %q", meta.Title, viewmodel.SiteName)
			}
			if meta.Description != tt.wantDescription {
				t.Errorf("Description = %q、期待値 = %q", meta.Description, tt.wantDescription)
			}
			if meta.AssetVersion != "abc1234" {
				t.Errorf("AssetVersion = %q、期待値 = %q", meta.AssetVersion, "abc1234")
			}
			if meta.OGLocale != tt.wantOGLocale {
				t.Errorf("OGLocale = %q、期待値 = %q", meta.OGLocale, tt.wantOGLocale)
			}
		})
	}
}

// TestDefaultPageMeta_CanonicalURL は各言語版が自分自身を正規のアドレスとして名乗ることを検証する。
// 片方をもう片方のcanonicalにすると、正規化された側が検索結果から落ちる。
func TestDefaultPageMeta_CanonicalURL(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	tests := []struct {
		name   string
		locale string
		path   string
		want   string
	}{
		{
			name:   "日本語版のトップ",
			locale: i18n.LangJa,
			path:   "/",
			want:   "https://cutre.example.com/",
		},
		{
			name:   "英語版のトップ",
			locale: i18n.LangEn,
			path:   "/",
			want:   "https://cutre.example.com/en",
		},
		{
			name:   "英語版の配下のページ",
			locale: i18n.LangEn,
			path:   "/items",
			want:   "https://cutre.example.com/en/items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			meta := viewmodel.DefaultPageMeta(ctx, cfg, tt.path)

			if meta.CanonicalURL != tt.want {
				t.Errorf("CanonicalURL = %q、期待値 = %q", meta.CanonicalURL, tt.want)
			}
		})
	}
}

// TestDefaultPageMeta_Alternates は全言語版が、表示中のロケールによらず同じ顔ぶれで並ぶことを検証する。
// 自己参照が欠けたり言語版ごとに顔ぶれが違ったりすると、相互参照が成立せずアノテーション全体が無視される。
//
// 言語スイッチャーが読む表記と現在地も同じ一覧から取るため、あわせて検証する。
func TestDefaultPageMeta_Alternates(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	for _, locale := range []string{i18n.LangJa, i18n.LangEn} {
		t.Run(locale, func(t *testing.T) {
			t.Parallel()

			want := []viewmodel.Alternate{
				{
					Lang:    i18n.LangJa,
					URL:     "https://cutre.example.com/",
					Endonym: "日本語",
					Current: locale == i18n.LangJa,
				},
				{
					Lang:    i18n.LangEn,
					URL:     "https://cutre.example.com/en",
					Endonym: "English",
					Current: locale == i18n.LangEn,
				},
			}

			ctx := i18n.SetLocale(context.Background(), locale)

			meta := viewmodel.DefaultPageMeta(ctx, cfg, "/")

			if !slices.Equal(meta.Alternates, want) {
				t.Errorf("Alternates = %v、期待値 = %v", meta.Alternates, want)
			}

			// hreflangが指すURLはそのページのcanonicalと一致していなければならない。
			// ずれていると、検索エンジンが辿った先のページが別のアドレスを正規として名乗ることになる。
			selfURL := ""
			for _, alternate := range meta.Alternates {
				if alternate.Lang == locale {
					selfURL = alternate.URL
				}
			}
			if selfURL != meta.CanonicalURL {
				t.Errorf("自己参照のalternateのURL = %q、期待値 = canonicalと同じ %q", selfURL, meta.CanonicalURL)
			}
		})
	}
}

// TestPageMeta_SetTitle はページ同士を見分けさせる部分が先に来て、サイト名が後ろに付くことを検証する。
// タイトル用のキー (welcome_show_title) はサイト名と同じ値のため、渡すとサフィックスの有無が
// 期待値に現れない。見出しのキーを借りて、付加されるサフィックスが見える形にする。
func TestPageMeta_SetTitle(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	var meta viewmodel.PageMeta
	meta.SetTitle(ctx, "welcome_show_heading")

	const want = "Cutreにようこそ！ | Cutre"
	if meta.Title != want {
		t.Errorf("Title = %q、期待値 = %q", meta.Title, want)
	}
}

// TestPageMeta_SetTitleWithoutSuffix はサフィックスを付けずにタイトルを設定できることを検証する。
// SetTitleと同じ理由で、サフィックスの有無が見えるよう見出しのキーを渡す。
func TestPageMeta_SetTitleWithoutSuffix(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangEn)

	var meta viewmodel.PageMeta
	meta.SetTitleWithoutSuffix(ctx, "welcome_show_heading")

	const want = "Welcome to Cutre!"
	if meta.Title != want {
		t.Errorf("Title = %q、期待値 = %q", meta.Title, want)
	}
}

// TestDefaultPageMeta_LocaleSuggestion は案内すべきロケールがあるときだけ案内が組み立てられ、
// その文言が案内先の言語になることを検証する。
//
// 表示中の言語で書くと、その言語を読めない利用者が案内に気付けない。
func TestDefaultPageMeta_LocaleSuggestion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	tests := []struct {
		name            string
		locale          string
		suggestedLocale string
		want            *viewmodel.LocaleSuggestion
	}{
		{
			name:            "英語版を見ている利用者に日本語版を案内する",
			locale:          i18n.LangEn,
			suggestedLocale: i18n.LangJa,
			want: &viewmodel.LocaleSuggestion{
				Lang:         i18n.LangJa,
				URL:          "https://cutre.example.com/",
				Label:        "言語版の案内",
				Message:      "このページは日本語でも読めます。",
				SwitchLabel:  "日本語で読む",
				DismissLabel: "閉じる",
				// 閉じる操作は表示中の言語版 (英語) を選んだものとして記録する。
				DismissLocale: i18n.LangEn,
			},
		},
		{
			name:            "日本語版を見ている利用者に英語版を案内する",
			locale:          i18n.LangJa,
			suggestedLocale: i18n.LangEn,
			want: &viewmodel.LocaleSuggestion{
				Lang:          i18n.LangEn,
				URL:           "https://cutre.example.com/en",
				Label:         "Language notice",
				Message:       "This page is also available in English.",
				SwitchLabel:   "Read in English",
				DismissLabel:  "Dismiss",
				DismissLocale: i18n.LangJa,
			},
		},
		{
			name:            "案内すべきロケールが無ければ組み立てない",
			locale:          i18n.LangJa,
			suggestedLocale: "",
			want:            nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)
			ctx = i18n.SetSuggestedLocale(ctx, tt.suggestedLocale)

			meta := viewmodel.DefaultPageMeta(ctx, cfg, "/")

			if tt.want == nil {
				if meta.LocaleSuggestion != nil {
					t.Errorf("LocaleSuggestion = %+v、期待値 = nil", meta.LocaleSuggestion)
				}
				return
			}

			if meta.LocaleSuggestion == nil {
				t.Fatal("LocaleSuggestion = nil、非nilを期待")
			}
			if *meta.LocaleSuggestion != *tt.want {
				t.Errorf("LocaleSuggestion = %+v、期待値 = %+v", *meta.LocaleSuggestion, *tt.want)
			}

			// 案内を組み立てても、ページ自身の描画に使うロケールは変わらない。
			if got := i18n.GetLocale(ctx); got != tt.locale {
				t.Errorf("表示中のロケール = %q、期待値 = %q", got, tt.locale)
			}
		})
	}
}

// TestErrorPageMeta はエラーページのメタ情報が、既定値とアセットバージョン・og:localeを持ちつつ、
// 存在しないアドレスを指すcanonical・hreflang・言語版の案内を持たないことを検証する。
func TestErrorPageMeta(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Env: "prod", Domain: "cutre.example.com", GitRev: "abc1234"}

	// 案内すべきロケールが載ったctxでも案内を組み立てないことを見るため、あえて別の言語を求めた状態にする。
	ctx := i18n.SetSuggestedLocale(i18n.SetLocale(context.Background(), i18n.LangEn), i18n.LangJa)

	meta := viewmodel.ErrorPageMeta(ctx, cfg)

	if meta.Title != "Cutre" {
		t.Errorf("Title = %q、期待値 = %q", meta.Title, "Cutre")
	}
	if meta.AssetVersion != "abc1234" {
		t.Errorf("AssetVersion = %q、期待値 = %q", meta.AssetVersion, "abc1234")
	}
	if meta.OGLocale != "en_US" {
		t.Errorf("OGLocale = %q、期待値 = %q", meta.OGLocale, "en_US")
	}

	if meta.CanonicalURL != "" {
		t.Errorf("CanonicalURL = %q、期待値 = 空文字列", meta.CanonicalURL)
	}
	if len(meta.Alternates) != 0 {
		t.Errorf("Alternatesの件数 = %d、期待値 = 0", len(meta.Alternates))
	}
	if meta.LocaleSuggestion != nil {
		t.Errorf("LocaleSuggestion = %+v、期待値 = nil", meta.LocaleSuggestion)
	}
}

// TestSignedInPageMeta は、ログイン後のページが言語版の参照も正規のアドレスも持たず、インデックスを断ることを検証する。
func TestSignedInPageMeta(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Env: "prod", Domain: "cutre.example.com", GitRev: "abc1234"}
	ctx := i18n.SetLocale(context.Background(), i18n.LangEn)

	meta := viewmodel.SignedInPageMeta(ctx, cfg)

	if !meta.NoIndex {
		t.Error("NoIndex = false、期待値 = true")
	}
	if meta.CanonicalURL != "" {
		t.Errorf("CanonicalURL = %q、期待値 = 空文字列", meta.CanonicalURL)
	}
	if len(meta.Alternates) != 0 {
		t.Errorf("Alternatesの件数 = %d、期待値 = 0", len(meta.Alternates))
	}
	if meta.OGLocale != "en_US" {
		t.Errorf("OGLocale = %q、期待値 = %q", meta.OGLocale, "en_US")
	}
}
