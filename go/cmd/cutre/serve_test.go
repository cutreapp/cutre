package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
)

// testConfig はルーターの組み立てに必要な最小限の設定を返す。
// Envをdevにするのは、アセットバージョンをgitコマンドの有無に左右されない値にするため。
func testConfig() *config.Config {
	return &config.Config{Env: "dev", Domain: "cutre.example.com"}
}

func TestNewRouter_Health(t *testing.T) {
	t.Parallel()

	router := newRouter(testConfig(), t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("レスポンスボディのデコードのエラー = %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q、期待値 = %q", body["status"], "ok")
	}
}

// TestNewRouter_Welcome は各言語版のトップページがそれぞれのパスに登録され、その言語のHTMLを返すことを固定する。
// ハンドラー単体のテストとは別に置くのは、登録先のパスを取り違えても
// welcome.Show 自体のテストは成功し、トップページが404になったことを検出できないため。
func TestNewRouter_Welcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		// 描画の中身は internal/handler/welcome のテストが持つため、ここでは
		// ルーターが返したものがどの言語版のトップページかを見出しだけで確かめる。
		wantHeading string
	}{
		{
			name:        "日本語版",
			path:        "/",
			wantHeading: "Cutreにようこそ！",
		},
		{
			name:        "英語版",
			path:        "/en",
			wantHeading: "Welcome to Cutre!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), t.TempDir())

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値 = text/htmlで始まる値", got)
			}

			if !strings.Contains(rec.Body.String(), tt.wantHeading) {
				t.Errorf("レスポンスボディに %qが含まれていない", tt.wantHeading)
			}
		})
	}
}

// TestNewRouter_I18n は newRouter が middleware.I18n を登録していることを固定する。
// ミドルウェア自体の挙動は internal/middleware のテストが持つため、ここで見るのは配線だけ。
func TestNewRouter_I18n(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		wantLocale string
	}{
		{
			name:       "英語版のパスのロケールがルーター経由でcontextに載る",
			path:       "/en/test-locale",
			wantLocale: i18n.LangEn,
		},
		{
			name:       "言語コードを持たないパスではデフォルトロケールが載る",
			path:       "/test-locale",
			wantLocale: i18n.DefaultLang,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// newRouter が返すルーターに検証用のルートを足し、登録済みのミドルウェアを
			// 通したあとのcontextを観測する。ルーターはテストごとに作り直すため共有されない。
			router := newRouter(testConfig(), t.TempDir())

			var gotLocale string
			observe := func(_ http.ResponseWriter, r *http.Request) {
				gotLocale = i18n.GetLocale(r.Context())
			}
			router.Get("/test-locale", observe)
			router.Get("/en/test-locale", observe)

			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.path, nil))

			if gotLocale != tt.wantLocale {
				t.Errorf("ロケール = %q、期待値 = %q", gotLocale, tt.wantLocale)
			}
		})
	}
}

// TestNewRouter_LocaleIndependentRoutes は言語版を持たないルートが、
// ロケールの判定に左右されずに同じ応答を返すことを固定する。
// これらは公開コンテンツではないため、言語版のURLを持たず、Accept-Languageでも内容が変わらない。
func TestNewRouter_LocaleIndependentRoutes(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "css"), 0o755); err != nil {
		t.Fatalf("テスト用ディレクトリの作成のエラー = %v", err)
	}
	const css = "body{color:red}"
	if err := os.WriteFile(filepath.Join(staticDir, "css", "style.css"), []byte(css), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "ヘルスチェック",
			path: "/health",
			want: `{"status":"ok"}`,
		},
		{
			name: "静的アセット",
			path: "/static/css/style.css",
			want: css,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), staticDir)

			for _, acceptLanguage := range []string{"", "en-US,en;q=0.9", "ja"} {
				req := httptest.NewRequest(http.MethodGet, tt.path, nil)
				if acceptLanguage != "" {
					req.Header.Set("Accept-Language", acceptLanguage)
				}
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("Accept-Language %qのステータスコード = %d、期待値 = %d", acceptLanguage, rec.Code, http.StatusOK)
				}
				if got := strings.TrimSpace(rec.Body.String()); got != tt.want {
					t.Errorf("Accept-Language %qのレスポンスボディ = %q、期待値 = %q", acceptLanguage, got, tt.want)
				}
			}
		})
	}
}

// TestNewRouter_StaticAssets は /static/* が配信元ディレクトリの中身を返すことを固定する。
// 配信元をテスト用のディレクトリにするのは、pnpm buildの生成物をGoのテストの前提にしないため。
func TestNewRouter_StaticAssets(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "css"), 0o755); err != nil {
		t.Fatalf("テスト用ディレクトリの作成のエラー = %v", err)
	}
	const want = "body{color:red}"
	if err := os.WriteFile(filepath.Join(staticDir, "css", "style.css"), []byte(want), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	router := newRouter(testConfig(), staticDir)

	req := httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Errorf("Content-Type = %q、期待値 = text/cssで始まる値", got)
	}

	if got := rec.Body.String(); got != want {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, want)
	}
}

// TestNewRouter_LocaleSuggestion は言語版の案内が、ミドルウェアの判定からページのHTMLまで
// 通しで届くことを固定する。
//
// 判定 (middleware) ・組み立て (viewmodel) ・描画 (templates) はそれぞれ単体のテストを持つが、
// どこか1つの配線が外れても各単体テストは成功する。
func TestNewRouter_LocaleSuggestion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		acceptLanguage string
		selectedLocale string
		wantMessage    string
	}{
		{
			name:           "日本語版を英語話者が見ると英語で案内が出る",
			path:           "/",
			acceptLanguage: "en-US,en;q=0.9",
			wantMessage:    "This page is also available in English.",
		},
		{
			name:           "英語版を日本語話者が見ると日本語で案内が出る",
			path:           "/en",
			acceptLanguage: "ja",
			wantMessage:    "このページは日本語でも読めます。",
		},
		{
			name:           "求める言語版を見ているときは案内が出ない",
			path:           "/en",
			acceptLanguage: "en",
			wantMessage:    "",
		},
		{
			name:           "選んだ言語版を見ているときは出ない",
			path:           "/",
			acceptLanguage: "en",
			selectedLocale: i18n.LangJa,
			wantMessage:    "",
		},
		{
			name:           "選んだ言語と違う言語版を開くと選んだ言語で案内が出る",
			path:           "/en",
			acceptLanguage: "en",
			selectedLocale: i18n.LangJa,
			wantMessage:    "このページは日本語でも読めます。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), t.TempDir())

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Language", tt.acceptLanguage)
			if tt.selectedLocale != "" {
				req.AddCookie(&http.Cookie{Name: middleware.SelectedLocaleCookie, Value: tt.selectedLocale})
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			body := rec.Body.String()

			if got := strings.Contains(body, "data-locale-suggestion"); got != (tt.wantMessage != "") {
				t.Errorf("案内の出力有無 = %t、期待値 = %t", got, tt.wantMessage != "")
			}
			if tt.wantMessage != "" && !strings.Contains(body, tt.wantMessage) {
				t.Errorf("レスポンスボディに %qが含まれていない", tt.wantMessage)
			}
		})
	}
}

// TestNewRouter_SecurityHeaders は newRouter を通じて、通常応答・静的アセット・
// panic時の500にセキュリティヘッダーが付くことを検証する。
// ミドルウェア単体のテストでは検出できない、ルーターへの登録漏れを防ぐため。
func TestNewRouter_SecurityHeaders(t *testing.T) {
	t.Parallel()

	// ルーター経由の各応答に期待するヘッダーの集合。
	wantHeaders := map[string]string{
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "frame-ancestors 'none'",
		"X-Frame-Options":         "DENY",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=(), payment=()",
	}

	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "style.css"), []byte("body{color:red}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "トップページ",
			path:       "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "静的アセット",
			path:       "/static/style.css",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ハンドラーがpanicしたときの500",
			path:       "/test-panic",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), staticDir)
			router.Get("/test-panic", func(_ http.ResponseWriter, _ *http.Request) {
				panic("テスト用のpanic")
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			resp := rec.Result()
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}

			for name, want := range wantHeaders {
				if got := resp.Header.Get(name); got != want {
					t.Errorf("%s = %q、期待値 = %q", name, got, want)
				}
			}
		})
	}
}

// TestNewRouter_NotFound は経路の無いパスが共通の404ページに落ちることを固定する。
// 描画そのもののテストは internal/httperror が持つため、ここで見るのは
// r.NotFound への登録と、ロケールを載せるミドルウェアより内側で描画されることの2点。
func TestNewRouter_NotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		wantHeading string
	}{
		{
			name:        "日本語版",
			path:        "/no-such-page",
			wantHeading: "ページが見つかりません",
		},
		{
			name:        "英語版",
			path:        "/en/no-such-page",
			wantHeading: "Page not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), t.TempDir())

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値 = text/htmlで始まる値", got)
			}

			if !strings.Contains(rec.Body.String(), tt.wantHeading) {
				t.Errorf("レスポンスボディに %qが含まれていない", tt.wantHeading)
			}
		})
	}
}

// TestNewRouter_MethodNotAllowed は経路はあるがメソッドを受け付けないリクエストが、
// 共通の405ページに落ち、許可メソッドを伴って返ることを固定する。
// 描画そのもののテストは internal/httperror が持つため、ここで見るのは
// r.MethodNotAllowed への登録と、Allowの付与と、ロケールを載せるミドルウェアより内側で描画されることの3点。
func TestNewRouter_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		path        string
		wantHeading string
	}{
		{
			name:        "日本語版のトップページへのPOST",
			method:      http.MethodPost,
			path:        "/",
			wantHeading: "この操作は利用できません",
		},
		{
			name:        "英語版のトップページへのPOST",
			method:      http.MethodPost,
			path:        "/en",
			wantHeading: "This action is not available",
		},
		{
			name:        "ヘルスチェックへのDELETE",
			method:      http.MethodDelete,
			path:        "/health",
			wantHeading: "この操作は利用できません",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), t.TempDir())

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMethodNotAllowed)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値 = text/htmlで始まる値", got)
			}

			// RFC 9110は405の応答に、そのアドレスが受け付けるメソッドの一覧を求めている。
			// HEADが入るのはGetHeadがGETのルートへフォールバックするため。
			wantAllow := []string{http.MethodGet, http.MethodHead}
			if got := rec.Header().Values("Allow"); !slices.Equal(got, wantAllow) {
				t.Errorf("Allow = %v、期待値 = %v", got, wantAllow)
			}

			if !strings.Contains(rec.Body.String(), tt.wantHeading) {
				t.Errorf("レスポンスボディに %qが含まれていない", tt.wantHeading)
			}
		})
	}
}

// TestAllowedMethods はAllowヘッダーの元になる許可メソッドが、ルーターの登録内容から求まることを検証する。
func TestAllowedMethods(t *testing.T) {
	t.Parallel()

	router := newRouter(testConfig(), t.TempDir())
	router.Head("/head-only", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Post("/post-only", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name string
		path string
		want []string
	}{
		// HEADが入るのはGetHeadがGETのルートへフォールバックするため。
		{name: "GETだけを登録したトップページ", path: "/", want: []string{http.MethodGet, http.MethodHead}},
		{name: "GETだけを登録した英語版のトップページ", path: "/en", want: []string{http.MethodGet, http.MethodHead}},
		{name: "GETだけを登録したヘルスチェック", path: "/health", want: []string{http.MethodGet, http.MethodHead}},
		{
			name: "メソッドを限定せず登録した静的アセット",
			path: "/static/css/style.css",
			want: allowProbeMethods,
		},
		{name: "HEADだけを登録したページ", path: "/head-only", want: []string{http.MethodHead}},
		{name: "POSTだけを登録したページ", path: "/post-only", want: []string{http.MethodPost}},
		{name: "経路の無いパス", path: "/no-such-page", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := allowedMethods(router, tt.path)

			if len(got) != len(tt.want) {
				t.Fatalf("許可メソッド = %v、期待値 = %v", got, tt.want)
			}

			for i, method := range tt.want {
				if got[i] != method {
					t.Errorf("許可メソッド[%d] = %q、期待値 = %q", i, got[i], method)
				}
			}
		})
	}
}

// TestNewRouter_Head はGETを登録したアドレスがHEADにも同じステータスで応えることを固定する。
// GetHeadによるフォールバックがルーター全体へ適用されていることを検証する。
func TestNewRouter_Head(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		path            string
		wantStatus      int
		wantContentType string
	}{
		{
			name:            "日本語版のトップページ",
			path:            "/",
			wantStatus:      http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
		},
		{
			name:            "英語版のトップページ",
			path:            "/en",
			wantStatus:      http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
		},
		{
			name:            "ヘルスチェック",
			path:            "/health",
			wantStatus:      http.StatusOK,
			wantContentType: "application/json",
		},
		{
			name:            "経路の無いパス",
			path:            "/no-such-page",
			wantStatus:      http.StatusNotFound,
			wantContentType: "text/html; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), t.TempDir())

			req := httptest.NewRequest(http.MethodHead, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}

			// HEADの応答はGETと同じヘッダーを持つ (RFC 9110)。
			if got := rec.Header().Get("Content-Type"); got != tt.wantContentType {
				t.Errorf("Content-Type = %q、期待値 = %q", got, tt.wantContentType)
			}
		})
	}
}

// TestNewRouter_HeadSendsNoBody はHEADの応答がボディを伴わないことを検証する。
//
// 生のコネクションへHEADを書いて応答をそのまま読む。
// httptest.NewRecorder はハンドラーが書いたボディを保持し、net/httpのHTTPクライアントは
// HEADの応答のボディを読まないため、どちらもボディが送られたかどうかを観測できない。
func TestNewRouter_HeadSendsNoBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(newRouter(testConfig(), t.TempDir()))
	t.Cleanup(server.Close)

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("接続のエラー = %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("接続のクローズのエラー = %v", err)
		}
	})

	request := "HEAD / HTTP/1.1\r\nHost: " + server.Listener.Addr().String() + "\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("リクエストの書き込みのエラー = %v", err)
	}

	raw, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("応答の読み取りのエラー = %v", err)
	}

	response := string(raw)

	if !strings.HasPrefix(response, "HTTP/1.1 200 OK\r\n") {
		t.Errorf("応答の先頭 = %q、期待値 = %q", firstLine(response), "HTTP/1.1 200 OK")
	}

	// ヘッダーの終端より後ろに何も続かないことが、ボディを送っていないこと。
	headerEnd := strings.Index(response, "\r\n\r\n")
	if headerEnd < 0 {
		t.Fatalf("応答にヘッダーの終端が無い: %q", response)
	}

	if body := response[headerEnd+len("\r\n\r\n"):]; body != "" {
		t.Errorf("レスポンスボディ = %q、期待値 = 空文字列", body)
	}

	// GETなら返すボディの長さは、HEADの応答でもContent-Lengthとして示してよい (RFC 9110)。
	if !strings.Contains(response, "Content-Type: text/html; charset=utf-8\r\n") {
		t.Errorf("応答にHTMLのContent-Typeが含まれていない: %q", response[:headerEnd])
	}
}

// firstLine は応答の1行目を返す。失敗メッセージに応答全体を並べないため。
func firstLine(response string) string {
	if i := strings.Index(response, "\r\n"); i >= 0 {
		return response[:i]
	}

	return response
}

// TestNewRouter_HeadRouting はGETへのフォールバックと明示したHEADのルートで、
// 後続へHEADのまま渡すことを検証する。ハンドラーのHEAD用の処理を有効に保つため。
func TestNewRouter_HeadRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		registerGet  bool
		registerHead bool
		wantHandler  string
	}{
		{name: "GETへのフォールバック", registerGet: true, wantHandler: "get"},
		{name: "明示したHEADを優先", registerGet: true, registerHead: true, wantHandler: "head"},
		{name: "HEADだけのルート", registerHead: true, wantHandler: "head"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			router := newRouter(testConfig(), t.TempDir())
			var gotHandler, gotMethod string
			if tt.registerGet {
				router.Get("/test-head", func(w http.ResponseWriter, r *http.Request) {
					gotHandler, gotMethod = "get", r.Method
					w.WriteHeader(http.StatusOK)
				})
			}
			if tt.registerHead {
				router.Head("/test-head", func(w http.ResponseWriter, r *http.Request) {
					gotHandler, gotMethod = "head", r.Method
					w.WriteHeader(http.StatusOK)
				})
			}
			req := httptest.NewRequest(http.MethodHead, "/test-head", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if gotHandler != tt.wantHandler {
				t.Errorf("呼ばれたハンドラー = %q、期待値 = %q", gotHandler, tt.wantHandler)
			}
			if gotMethod != http.MethodHead {
				t.Errorf("後続のメソッド = %q、期待値 = %q", gotMethod, http.MethodHead)
			}
			if req.Method != http.MethodHead {
				t.Errorf("受け取ったリクエストのメソッド = %q、期待値 = %q", req.Method, http.MethodHead)
			}
		})
	}
}

// TestNewRouter_HeadStaticAsset は静的配信がHEADで本文を書き出さないことを検証する。
// Recorderは本文を捨てないため、ネットワーク層が抑止する前のFileServerの動作を確認できる。
func TestNewRouter_HeadStaticAsset(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	content := strings.Repeat(" ", 1<<20)
	if err := os.WriteFile(filepath.Join(staticDir, "sample.css"), []byte(content), 0600); err != nil {
		t.Fatalf("静的ファイルの作成のエラー = %v", err)
	}
	router := newRouter(testConfig(), staticDir)
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/static/sample.css", nil))
	if get.Code != http.StatusOK || get.Body.String() != content {
		t.Fatalf("GETの静的配信が期待どおりでない: ステータス = %d、本文の長さ = %d", get.Code, get.Body.Len())
	}
	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/static/sample.css", nil))
	if head.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", head.Code, http.StatusOK)
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Last-Modified", "Accept-Ranges"} {
		if got, want := head.Header().Get(name), get.Header().Get(name); got != want {
			t.Errorf("%s = %q、期待値 = %q", name, got, want)
		}
	}
	if head.Body.Len() != 0 {
		t.Errorf("FileServerが書き出した本文の長さ = %d、期待値 = 0", head.Body.Len())
	}
}

// TestNewRouter_RedirectSlashes は末尾スラッシュ付きのURLが、スラッシュ無しの同じURLへ
// 301で正規化され、その301が発行元を名乗ることを固定する。
//
// 正規化とRedirect-Byはどちらもミドルウェア単体のテストを持つが、
// ルーターへの登録漏れや登録順の入れ替わりはそこでは検出できない。
func TestNewRouter_RedirectSlashes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		wantLocation string
	}{
		{
			name:         "ヘルスチェック",
			path:         "/health/",
			wantLocation: "/health",
		},
		{
			name:         "英語版のトップページ",
			path:         "/en/",
			wantLocation: "/en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), t.TempDir())

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != http.StatusMovedPermanently {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMovedPermanently)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Fatalf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
			if got := rec.Header().Get("Redirect-By"); got != "cutre" {
				t.Errorf("Redirect-By = %q、期待値 = %q", got, "cutre")
			}

			// 転送先が本当に応答することまで見る。Locationの文字列が合っていても、
			// 正規化した形にルートが無ければ訪問者は404に着く。
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.wantLocation, nil))

			if rec.Code != http.StatusOK {
				t.Errorf("%sのステータスコード = %d、期待値 = %d", tt.wantLocation, rec.Code, http.StatusOK)
			}
		})
	}
}

// TestNewRouter_NoRedirectForNormalizedPaths は、正規化するものが無いリクエストが
// リダイレクトされず、発行元のヘッダーも持たないことを固定する。
//
// トップページを含めるのは、パスが "/" の1文字だけで末尾スラッシュと見分けが付かず、
// 正規化の条件を誤ると自分自身へのリダイレクトで無限ループになるため。
func TestNewRouter_NoRedirectForNormalizedPaths(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "css"), 0o755); err != nil {
		t.Fatalf("テスト用ディレクトリの作成のエラー = %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "css", "style.css"), []byte("body{color:red}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	for _, path := range []string{"/", "/en", "/health", "/static/css/style.css"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), staticDir)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Redirect-By"); got != "" {
				t.Errorf("Redirect-By = %q、期待値 = 付かないこと", got)
			}
		})
	}
}

// TestNewRouter_AssetDirectoryDoesNotLoop は、静的アセットを収めたディレクトリが
// URLの2つの形の間を往復するのではなく404に落ち着くことを固定する。
//
// http.FileServer はディレクトリのパスに末尾スラッシュを足すリダイレクトを返し、
// middleware.RedirectSlashes はそれを剥がす。2つを素で組み合わせると訪問者は無限ループを踏む。
// Cutreが免れているのは配信元がディレクトリを存在しないものとして扱うためで (assetFileSystem)、
// それが成り立たなくなったことに気付くのが本テストである。
func TestNewRouter_AssetDirectoryDoesNotLoop(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "css"), 0o755); err != nil {
		t.Fatalf("テスト用ディレクトリの作成のエラー = %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "css", "style.css"), []byte("body{color:red}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	router := newRouter(testConfig(), staticDir)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMovedPermanently)
	}
	location := rec.Header().Get("Location")
	if location != "/static/css" {
		t.Fatalf("Location = %q、期待値 = %q", location, "/static/css")
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, location, nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("%sのステータスコード = %d、期待値 = %d", location, rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("%sのLocation = %q、期待値 = 付かないこと", location, got)
	}
}

// TestNewRouter_RedirectEscapedAssets は予約文字を含むアセットの転送先を
// 実クライアントで辿り、ファイル名とクエリが同じまま取得できることを検証する。
func TestNewRouter_RedirectEscapedAssets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filename    string
		escapedName string
	}{
		{name: "ハッシュ", filename: "a#b.css", escapedName: "a%23b.css"},
		{name: "疑問符", filename: "a?b.css", escapedName: "a%3Fb.css"},
		{name: "パーセント", filename: "a%b.css", escapedName: "a%25b.css"},
		{name: "バックスラッシュ", filename: "a\\b.css", escapedName: "a%5Cb.css"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			staticDir := t.TempDir()
			const wantBody = "body{color:red}"
			if err := os.WriteFile(filepath.Join(staticDir, tt.filename), []byte(wantBody), 0o600); err != nil {
				t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
			}
			server := httptest.NewServer(newRouter(testConfig(), staticDir))
			t.Cleanup(server.Close)

			const query = "v=1&tag=a%26b&tag=c+d"
			wantLocation := "/static/" + tt.escapedName + "?" + query
			client := server.Client()
			redirects := 0
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				redirects++
				if len(via) > 1 {
					return http.ErrUseLastResponse
				}
				if got := req.Response.StatusCode; got != http.StatusMovedPermanently {
					t.Errorf("転送ステータス = %d、期待値 = %d", got, http.StatusMovedPermanently)
				}
				if got := req.Response.Header.Get("Location"); got != wantLocation {
					t.Errorf("Location = %q、期待値 = %q", got, wantLocation)
				}
				if got := req.Response.Header.Get("Redirect-By"); got != "cutre" {
					t.Errorf("Redirect-By = %q、期待値 = cutre", got)
				}
				return nil
			}

			resp, err := client.Get(server.URL + "/static/" + tt.escapedName + "/?" + query)
			if err != nil {
				t.Fatalf("アセット取得のエラー = %v", err)
			}
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if redirects != 1 {
				t.Errorf("転送回数 = %d、期待値 = 1", redirects)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("最終ステータス = %d、期待値 = %d", resp.StatusCode, http.StatusOK)
			}
			if got, want := resp.Request.URL.Path, "/static/"+tt.filename; got != want {
				t.Errorf("最終パス = %q、期待値 = %q", got, want)
			}
			if got := resp.Request.URL.RawQuery; got != query {
				t.Errorf("最終クエリ = %q、期待値 = %q", got, query)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("本文の読み取りのエラー = %v", err)
			}
			if string(body) != wantBody {
				t.Errorf("本文 = %q、期待値 = %q", body, wantBody)
			}
		})
	}
}

// TestNewRouter_CacheControl は、ルーター経由の各応答が明示のキャッシュ方針を持ち、
// 言語設定による分岐の申告がHTMLにだけ付くことを固定する。
//
// ミドルウェア単体のテストとは別に置くのは、方針が2つのミドルウェアと
// エラーページの取り合わせで決まり、登録の順序を変えると結果が変わるため。
func TestNewRouter_CacheControl(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "style.css"), []byte("body{color:red}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	tests := []struct {
		name             string
		env              string
		path             string
		rangeHeader      string
		wantStatus       int
		wantCacheControl string
		wantVary         string
	}{
		{
			name:             "トップページ",
			env:              "dev",
			path:             "/",
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-cache",
			wantVary:         "Accept-Language",
		},
		{
			name:             "静的アセット",
			env:              "prod",
			path:             "/static/style.css",
			wantStatus:       http.StatusOK,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name:             "開発環境の静的アセット",
			env:              "dev",
			path:             "/static/style.css",
			wantStatus:       http.StatusOK,
			wantCacheControl: "no-store",
		},
		{
			name:             "静的アセットの一部だけを求めるリクエスト",
			env:              "prod",
			path:             "/static/style.css",
			rangeHeader:      "bytes=0-4",
			wantStatus:       http.StatusPartialContent,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name:             "存在しないアセット",
			env:              "prod",
			path:             "/static/missing.css",
			wantStatus:       http.StatusNotFound,
			wantCacheControl: "private, no-store",
		},
		{
			name:             "404ページ",
			env:              "dev",
			path:             "/missing",
			wantStatus:       http.StatusNotFound,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name:             "末尾スラッシュの正規化",
			env:              "dev",
			path:             "/health/",
			wantStatus:       http.StatusMovedPermanently,
			wantCacheControl: "private, no-cache",
			wantVary:         "Accept-Language",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := testConfig()
			cfg.Env = tt.env

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.rangeHeader != "" {
				req.Header.Set("Range", tt.rangeHeader)
			}

			rec := httptest.NewRecorder()
			newRouter(cfg, staticDir).ServeHTTP(rec, req)
			resp := rec.Result()
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := resp.Header.Values("Cache-Control"); len(got) != 1 || got[0] != tt.wantCacheControl {
				t.Errorf("Cache-Control = %v、期待値 = [%q]", got, tt.wantCacheControl)
			}

			values := resp.Header.Values("Vary")
			if tt.wantVary == "" {
				if len(values) != 0 {
					t.Errorf("Vary = %v、期待値 = 無し", values)
				}
				return
			}
			if len(values) != 1 || values[0] != tt.wantVary {
				t.Errorf("Vary = %v、期待値 = [%q]", values, tt.wantVary)
			}
		})
	}
}
