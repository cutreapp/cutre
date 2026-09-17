package welcome_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/welcome"
	"github.com/cutreapp/cutre/go/internal/i18n"
)

// TestShow はトップページが200とHTMLを返し、contextのロケールでlang属性・タイトル・見出し・説明文・
// og:localeが切り替わること、そして自分の言語版のcanonicalとバージョン付きのアセット参照を持つことを検証する。
// ロケールはミドルウェアを通さずcontextへ直接載せ、ハンドラーだけを動かす。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := welcome.NewHandler(&config.Config{Env: "dev", Domain: "cutre.example.com"})

	tests := []struct {
		name          string
		path          string
		locale        string
		wantTitle     string
		wantHeading   string
		wantLead      string
		wantSkip      string
		wantOGLocale  string
		wantCanonical string
	}{
		{
			name:          "日本語",
			path:          "/",
			locale:        i18n.LangJa,
			wantTitle:     "<title>Cutre</title>",
			wantHeading:   "Cutreにようこそ！",
			wantLead:      "キャラクターグッズの物々交換を、もっと簡単に。",
			wantSkip:      "メインコンテンツへスキップ",
			wantOGLocale:  "ja_JP",
			wantCanonical: "https://cutre.example.com/",
		},
		{
			name:          "英語",
			path:          "/en",
			locale:        i18n.LangEn,
			wantTitle:     "<title>Cutre</title>",
			wantHeading:   "Welcome to Cutre!",
			wantLead:      "Trading character goods, made simpler.",
			wantSkip:      "Skip to main content",
			wantOGLocale:  "en_US",
			wantCanonical: "https://cutre.example.com/en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値 = text/htmlで始まる値", got)
			}

			body := rec.Body.String()
			wants := []string{
				`<html lang="` + tt.locale + `">`,
				tt.wantTitle,
				tt.wantLead,
				tt.wantSkip,
				`<link rel="canonical" href="` + tt.wantCanonical + `">`,
				`<meta property="og:url" content="` + tt.wantCanonical + `">`,
				`<meta property="og:site_name" content="Cutre">`,
				`<meta property="og:locale" content="` + tt.wantOGLocale + `">`,
				// 言語版どうしの対応は、どちらの言語版を見ていても同じ顔ぶれで並ぶ。
				`<link rel="alternate" hreflang="ja" href="https://cutre.example.com/">`,
				`<link rel="alternate" hreflang="en" href="https://cutre.example.com/en">`,
				`<main id="main" tabindex="-1"`,
				"<footer",
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %qが含まれていない", want)
				}
			}

			// 見出しは<h1>の中にあって初めてページの主題になる。
			// 本文のどこかに同じ文字列があることだけを見ると、<h1>が空になった回帰を見逃す。
			heading := regexp.MustCompile(`<h1[^>]*>` + regexp.QuoteMeta(tt.wantHeading) + `</h1>`)
			if !heading.MatchString(body) {
				t.Errorf("レスポンスボディの<h1>の中身が %qになっていない", tt.wantHeading)
			}

			// ?v= は値が入って初めてキャッシュを切り替えられるため、後ろが空でないことまで見る。
			assetRefs := []*regexp.Regexp{
				regexp.MustCompile(`href="/static/css/style\.css\?v=[^"]+"`),
				regexp.MustCompile(`src="/static/js/main\.js\?v=[^"]+"`),
			}
			for _, assetRef := range assetRefs {
				if !assetRef.MatchString(body) {
					t.Errorf("レスポンスボディが %vに一致しない", assetRef)
				}
			}
		})
	}
}

// TestShow_Description はmeta descriptionとog:descriptionがトップページ固有の文言になることを検証する。
// DefaultPageMeta が返すサイト全体の既定値のままだと、どのページも同じ要約を名乗ることになる。
// 英語の既定値はトップページの文言と同じ文で始まり、差が末尾にしか出ないため、両方のロケールを回す。
func TestShow_Description(t *testing.T) {
	t.Parallel()

	handler := welcome.NewHandler(&config.Config{Env: "dev", Domain: "cutre.example.com"})

	tests := []struct {
		name            string
		path            string
		locale          string
		wantDescription string
	}{
		{
			name:            "日本語",
			path:            "/",
			locale:          i18n.LangJa,
			wantDescription: "Cutreはキャラクターグッズの物々交換を簡単にするサービスです。欲しいグッズと譲れるグッズを登録して、交換相手を見つけられます",
		},
		{
			name:            "英語",
			path:            "/en",
			locale:          i18n.LangEn,
			wantDescription: "Cutre makes trading character goods simple. Register the goods you want and the ones you can part with, and find someone to trade with.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			body := rec.Body.String()
			wants := []string{
				`<meta name="description" content="` + tt.wantDescription + `">`,
				`<meta property="og:description" content="` + tt.wantDescription + `">`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %qが含まれていない", want)
				}
			}
		})
	}
}
