package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestHTMLCache は、HTMLのレスポンスにだけ言語設定による分岐と既定のキャッシュ方針が付くことを検証する。
//
// HTMLかどうかはContent-Typeで判定し、未設定ならエンコードされていない本文から判定する。
// Content-Typeを設定した応答では、ステータスと本文のどちらから書き出しても方針が付くことを固定する。
func TestHTMLCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		handler          http.Handler
		wantStatus       int
		wantCacheControl string
		wantVary         string
	}{
		{
			name: "ステータスを設定するHTMLに方針が付く",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			}),
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-cache",
			wantVary:         "Accept-Language",
		},
		{
			name: "ステータスを設定せず本文だけを書くHTMLにも方針が付く",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if _, err := w.Write([]byte("<!doctype html>")); err != nil {
					t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
				}
			}),
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-cache",
			wantVary:         "Accept-Language",
		},
		{
			name: "ハンドラーが決めたキャッシュ方針は残る",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "private, no-store")
				w.WriteHeader(http.StatusNotFound)
			}),
			wantStatus:       http.StatusNotFound,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name: "HTMLを伴うリダイレクトにも方針が付く",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/health", http.StatusMovedPermanently)
			}),
			wantStatus:       http.StatusMovedPermanently,
			wantCacheControl: "private, no-cache",
			wantVary:         "Accept-Language",
		},
		{
			name: "HTML以外には付かない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			}),
			wantStatus: http.StatusOK,
		},
		{
			name: "何も名乗らない応答には付かない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "型を名乗らずHTML以外を書く応答には付かない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("ただの文章")); err != nil {
					t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
				}
			}),
			wantStatus: http.StatusOK,
		},
		{
			name: "エンコード済みの本文からは判定しない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				if _, err := w.Write([]byte("<!doctype html><html><body>hello</body></html>")); err != nil {
					t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
				}
			}),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			middleware.HTMLCache(tt.handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			resp := rec.Result()
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := resp.Header.Get("Cache-Control"); got != tt.wantCacheControl {
				t.Errorf("Cache-Control = %q、期待値 = %q", got, tt.wantCacheControl)
			}
			// Header.Getだけでは重複を検出できないため、値の個数も見て、
			// Accept-Languageが1回だけ追加されることを検証する。
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

// TestHTMLCache_KeepsExistingVary は、ほかの要因で表現が変わる応答のVaryを置き換えないことを検証する。
// 列挙を落とすと、その要因で分岐した表現が取り違えて配られる。
func TestHTMLCache_KeepsExistingVary(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	handler := middleware.HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"Accept-Encoding", "Accept-Language"}
	got := rec.Result().Header.Values("Vary")
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Vary = %v、期待値 = %v", got, want)
	}
}

// TestHTMLCache_KeepsSingleAcceptLanguage は、ハンドラーが自分で言語設定を列挙した応答に、
// 同じフィールド名を重ねないことを検証する。
// 重複はキャッシュのキーを変えず、列挙を読む側に同じ応答が違う応答として見えるだけになる。
func TestHTMLCache_KeepsSingleAcceptLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		vary string
		want []string
	}{
		{name: "同じ表記で設定済み", vary: "Accept-Language", want: []string{"Accept-Language"}},
		{name: "大文字小文字が異なる表記で設定済み", vary: "accept-language", want: []string{"accept-language"}},
		{name: "ほかの要因と一緒に設定済み", vary: "Accept-Encoding, Accept-Language", want: []string{"Accept-Encoding, Accept-Language"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler := middleware.HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Vary", tt.vary)
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			got := rec.Result().Header.Values("Vary")
			if len(got) != len(tt.want) || got[0] != tt.want[0] {
				t.Errorf("Vary = %v、期待値 = %v", got, tt.want)
			}
		})
	}
}

// TestHTMLCache_DetectsTypeFromBody は、Content-Typeを名乗らないハンドラーのHTMLにも方針が付くことを検証する。
//
// net/httpが実際に送信するContent-Typeとキャッシュヘッダーを、
// 実サーバーのクライアント側で受信して確かめる。
func TestHTMLCache_DetectsTypeFromBody(t *testing.T) {
	t.Parallel()

	const wantBody = "<!doctype html><html><body>hello</body></html>"
	handler := middleware.HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(wantBody)); err != nil {
			t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
		}
	}))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	resp, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("リクエストの送信のエラー = %v", err)
	}
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("レスポンスボディのクローズのエラー = %v", err)
		}
	})

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("レスポンスボディの読み込みのエラー = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", resp.StatusCode, http.StatusOK)
	}
	// net/httpが名乗った型と、このミドルウェアが判定した型が一致していることを見る。
	if got := resp.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "text/html; charset=utf-8")
	}
	if got := resp.Header.Values("Cache-Control"); len(got) != 1 || got[0] != "private, no-cache" {
		t.Errorf("Cache-Control = %v、期待値 = [private, no-cache]", got)
	}
	if got := resp.Header.Values("Vary"); len(got) != 1 || got[0] != "Accept-Language" {
		t.Errorf("Vary = %v、期待値 = [Accept-Language]", got)
	}
	if string(body) != wantBody {
		t.Errorf("本文 = %q、期待値 = %q", body, wantBody)
	}
}

// TestHTMLCache_ResponseHeaders は、先行Flushや情報応答を挟んでも、最終レスポンスに方針が付くことを検証する。
// サーバーがヘッダーを送信するタイミングを再現し、送信後のHeaderマップではなくクライアントの受信値を見る。
func TestHTMLCache_ResponseHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		earlyHints bool
		flush      bool
	}{
		{name: "本文より先にFlushする", flush: true},
		{name: "103の後にHTMLを返す", earlyHints: true},
		{name: "103の後にFlushしてHTMLを返す", earlyHints: true, flush: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const wantBody = "<!doctype html><html><body>hello</body></html>"
			handler := middleware.HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.earlyHints {
					w.WriteHeader(http.StatusEarlyHints)
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Vary", "Accept-Encoding")
				if tt.flush {
					if err := http.NewResponseController(w).Flush(); err != nil {
						t.Errorf("レスポンスのフラッシュのエラー = %v", err)
						return
					}
				} else {
					w.WriteHeader(http.StatusOK)
				}
				if _, err := w.Write([]byte(wantBody)); err != nil {
					t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
				}
			}))
			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)

			// 最終応答だけを確認すると、103自体を捨てる実装でもテストが通ってしまう。
			var informationalStatuses []int
			trace := &httptrace.ClientTrace{
				Got1xxResponse: func(code int, _ textproto.MIMEHeader) error {
					informationalStatuses = append(informationalStatuses, code)
					return nil
				},
			}
			req, err := http.NewRequestWithContext(httptrace.WithClientTrace(t.Context(), trace), http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatalf("リクエストの作成のエラー = %v", err)
			}
			resp, err := server.Client().Do(req)
			if err != nil {
				t.Fatalf("リクエストの送信のエラー = %v", err)
			}
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("レスポンスボディの読み込みのエラー = %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", resp.StatusCode, http.StatusOK)
			}
			if got := resp.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q、期待値 = %q", got, "text/html; charset=utf-8")
			}
			if got := resp.Header.Values("Cache-Control"); len(got) != 1 || got[0] != "private, no-cache" {
				t.Errorf("Cache-Control = %v、期待値 = [private, no-cache]", got)
			}
			if got := resp.Header.Values("Vary"); len(got) != 2 || got[0] != "Accept-Encoding" || got[1] != "Accept-Language" {
				t.Errorf("Vary = %v、期待値 = [Accept-Encoding Accept-Language]", got)
			}
			if string(body) != wantBody {
				t.Errorf("本文 = %q、期待値 = %q", body, wantBody)
			}
			if tt.earlyHints {
				if len(informationalStatuses) != 1 || informationalStatuses[0] != http.StatusEarlyHints {
					t.Errorf("情報応答 = %v、期待値 = [103]", informationalStatuses)
				}
			} else if len(informationalStatuses) != 0 {
				t.Errorf("情報応答 = %v、期待値 = 無し", informationalStatuses)
			}
		})
	}
}

// TestHTMLCache_UsesUnderlyingReadFrom は、本文の書き出しが下層のResponseWriterの
// io.ReaderFrom へ渡ることを検証する。
//
// net/httpのResponseWriterはそこでsendfileを使い、静的アセットの配信がこの経路を通る。
// このラッパーが io.ReaderFrom を隠すと io.Copy から見えなくなり、経路が変わる。
func TestHTMLCache_UsesUnderlyingReadFrom(t *testing.T) {
	t.Parallel()

	const wantBody = "body{color:red}"

	rec := &readFromRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler := middleware.HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")

		// http.ServeContentがio.CopyNでファイルを渡す形に合わせる。
		// strings.ReaderのWriteToが先に使われると、ReadFromを通る経路にならない。
		body := io.LimitReader(strings.NewReader(wantBody), int64(len(wantBody)))
		if _, err := io.Copy(w, body); err != nil {
			t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil))

	if !rec.readFromCalled {
		t.Error("下層のio.ReaderFromへ到達できなかった")
	}
	if got := rec.Body.String(); got != wantBody {
		t.Errorf("本文 = %q、期待値 = %q", got, wantBody)
	}
}
