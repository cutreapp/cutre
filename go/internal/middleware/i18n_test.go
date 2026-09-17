package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
)

func TestI18n(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		wantLocale     string
		wantTranslated string
	}{
		{
			name:           "言語コードを持たないパスではデフォルトロケールが載る",
			path:           "/",
			wantLocale:     i18n.DefaultLang,
			wantTranslated: "メインコンテンツへスキップ",
		},
		{
			name:           "英語版のパスでは英語が載る",
			path:           "/en",
			wantLocale:     i18n.LangEn,
			wantTranslated: "Skip to main content",
		},
		{
			name:           "英語版の配下のパスでも英語が載る",
			path:           "/en/items",
			wantLocale:     i18n.LangEn,
			wantTranslated: "Skip to main content",
		},
		{
			name:           "言語版を持たないパスではデフォルトロケールが載る",
			path:           "/health",
			wantLocale:     i18n.DefaultLang,
			wantTranslated: "メインコンテンツへスキップ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotLocale, gotTranslated string
			handler := middleware.I18n(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotLocale = i18n.GetLocale(r.Context())
				gotTranslated = i18n.T(r.Context(), "skip_to_main_content")
			}))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.path, nil))

			if gotLocale != tt.wantLocale {
				t.Errorf("ロケール = %q、期待値 = %q", gotLocale, tt.wantLocale)
			}
			// ロケールと一緒にLocalizerも載るため、後続で解決する翻訳が同じ言語になる。
			if gotTranslated != tt.wantTranslated {
				t.Errorf("翻訳 = %q、期待値 = %q", gotTranslated, tt.wantTranslated)
			}
		})
	}
}

// TestI18n_IgnoresAcceptLanguage は表示する言語を開いたURLだけで決めることを固定する。
// Accept-Languageで切り替えると、利用者が開いたURLと違う言語版の内容が同じURLから返る。
func TestI18n_IgnoresAcceptLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		acceptLanguage string
		wantLocale     string
	}{
		{
			name:           "日本語版のURLは英語を要求されても日本語のまま",
			path:           "/",
			acceptLanguage: "en-US,en;q=0.9",
			wantLocale:     i18n.LangJa,
		},
		{
			name:           "英語版のURLは日本語を要求されても英語のまま",
			path:           "/en",
			acceptLanguage: "ja",
			wantLocale:     i18n.LangEn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotLocale string
			handler := middleware.I18n(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotLocale = i18n.GetLocale(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Language", tt.acceptLanguage)

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if gotLocale != tt.wantLocale {
				t.Errorf("ロケール = %q、期待値 = %q", gotLocale, tt.wantLocale)
			}
		})
	}
}

// TestI18n_SuggestedLocale はブラウザが求める言語と表示中の言語版が食い違うときだけ、
// 別の言語版の案内をcontextに載せることを検証する。
//
// Accept-Languageは案内を出すかどうかの判定にだけ使い、表示する言語は切り替えない
// (TestI18n_IgnoresAcceptLanguageを参照)。
func TestI18n_SuggestedLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		acceptLanguage string
		want           string
	}{
		{
			name:           "日本語版を英語話者が見ているときは英語版を案内する",
			path:           "/",
			acceptLanguage: "en-US,en;q=0.9",
			want:           i18n.LangEn,
		},
		{
			name:           "英語版を日本語話者が見ているときは日本語版を案内する",
			path:           "/en",
			acceptLanguage: "ja",
			want:           i18n.LangJa,
		},
		{
			name:           "求める言語と表示中の言語版が同じなら案内しない",
			path:           "/",
			acceptLanguage: "ja,en;q=0.5",
			want:           "",
		},
		{
			name:           "翻訳を持たない言語を求めていても案内しない",
			path:           "/en",
			acceptLanguage: "fr-FR,de;q=0.9",
			want:           "",
		},
		{
			name:           "Accept-Languageが無ければ案内しない",
			path:           "/en",
			acceptLanguage: "",
			want:           "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got string
			handler := middleware.I18n(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = i18n.GetSuggestedLocale(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if got != tt.want {
				t.Errorf("案内すべきロケール = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestI18n_SuggestedLocaleSelected は利用者が明示的に選んだ言語を、ブラウザの言語設定より
// 優先して案内の判定に使うことを検証する。
//
// 推定を優先すると、スイッチャーで相手言語版へ移った利用者に元の言語版への案内が出続ける。
// 案内を閉じる操作も「表示中の言語版を選んだ」ものとして同じCookieに記録されるため、
// 閉じたあとに案内が出ないことも本テストが押さえている。
func TestI18n_SuggestedLocaleSelected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		acceptLanguage string
		selectedLocale string
		want           string
	}{
		{
			name:           "選んだ言語版を見ているときは案内しない",
			path:           "/",
			acceptLanguage: "en-US,en;q=0.9",
			selectedLocale: i18n.LangJa,
			want:           "",
		},
		{
			name:           "選んだ言語と違う言語版を開いたときは選んだ言語版を案内する",
			path:           "/en",
			acceptLanguage: "en-US,en;q=0.9",
			selectedLocale: i18n.LangJa,
			want:           i18n.LangJa,
		},
		{
			name:           "ブラウザの言語設定より選んだ言語が優先される",
			path:           "/",
			acceptLanguage: "ja",
			selectedLocale: i18n.LangEn,
			want:           i18n.LangEn,
		},
		{
			name:           "翻訳を持たないロケールが書かれていたらブラウザの言語設定に戻る",
			path:           "/",
			acceptLanguage: "en",
			selectedLocale: "fr",
			want:           i18n.LangEn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got string
			handler := middleware.I18n(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = i18n.GetSuggestedLocale(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Language", tt.acceptLanguage)
			req.AddCookie(&http.Cookie{Name: middleware.SelectedLocaleCookie, Value: tt.selectedLocale})

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if got != tt.want {
				t.Errorf("案内すべきロケール = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestI18n_SelectedLocaleDoesNotChangeDisplayedLocale は、選んだ言語を記録していても
// 開いたURLの言語版をそのまま返すことを検証する。
//
// 記録した選択で表示を差し替えると、共有されたリンクの行き先が受け手ごとに変わる。
func TestI18n_SelectedLocaleDoesNotChangeDisplayedLocale(t *testing.T) {
	t.Parallel()

	var gotLocale string
	handler := middleware.I18n(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotLocale = i18n.GetLocale(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middleware.SelectedLocaleCookie, Value: i18n.LangEn})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotLocale != i18n.LangJa {
		t.Errorf("ロケール = %q、期待値 = %q", gotLocale, i18n.LangJa)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Locationヘッダー = %q、期待値 = 空文字列", got)
	}
}
