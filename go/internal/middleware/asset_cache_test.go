package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestAssetCache は、静的アセットの配信に環境と結果に応じたキャッシュ方針が付くことを検証する。
//
// 長期の保存を許せるのはアセットを実際に配れたときだけで、それ以外の応答を同じ扱いにすると、
// 一時的な失敗が1年間そのアドレスに残り続ける。
func TestAssetCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		env              string
		handler          http.Handler
		wantStatus       int
		wantCacheControl string
	}{
		{
			name: "配信できたアセットは長く保存してよい",
			env:  "prod",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			}),
			wantStatus:       http.StatusOK,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name: "開発環境では保存しない",
			env:  "dev",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			}),
			wantStatus:       http.StatusOK,
			wantCacheControl: "no-store",
		},
		{
			name: "ステータスを設定せず本文だけを書く配信にも方針が付く",
			env:  "prod",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("body{color:red}")); err != nil {
					t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
				}
			}),
			wantStatus:       http.StatusOK,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name: "保存済みのアセットをそのまま使ってよい応答は方針を保つ",
			env:  "prod",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotModified)
			}),
			wantStatus:       http.StatusNotModified,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name: "表現の一部を配れた応答は方針を保つ",
			env:  "prod",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusPartialContent)
			}),
			wantStatus:       http.StatusPartialContent,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name: "求められたレンジを返せなかった応答は保存させない",
			env:  "prod",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			}),
			wantStatus:       http.StatusRequestedRangeNotSatisfiable,
			wantCacheControl: "private, no-store",
		},
		{
			name:             "存在しないアセットへの応答は保存させない",
			env:              "prod",
			handler:          http.NotFoundHandler(),
			wantStatus:       http.StatusNotFound,
			wantCacheControl: "private, no-store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{Env: tt.env}

			rec := httptest.NewRecorder()
			handler := middleware.AssetCache(cfg)(tt.handler)
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil))
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
		})
	}
}

// TestAssetCache_UsesUnderlyingReadFrom は、本文の書き出しが下層のResponseWriterの
// io.ReaderFrom へ渡ることを検証する。
//
// net/httpのResponseWriterはそこでsendfileを使い、アセットの配信がこの経路を通る。
// このラッパーが io.ReaderFrom を隠すと io.Copy から見えなくなり、経路が変わる。
func TestAssetCache_UsesUnderlyingReadFrom(t *testing.T) {
	t.Parallel()

	const wantBody = "body{color:red}"

	rec := &readFromRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler := middleware.AssetCache(&config.Config{Env: "prod"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

// TestAssetCache_RangeRequest は、Rangeヘッダーを伴う配信でも長期の方針が保たれることを、
// 実際の配信元である http.FileServer 越しに検証する。
//
// 206を返すのはこのミドルウェアではなくFileServerであり、
// 単体のハンドラーを並べた表では、その応答がどこから来るのかが残らない。
func TestAssetCache_RangeRequest(t *testing.T) {
	t.Parallel()

	const body = "body{color:red}body{color:blue}"

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "style.css"), []byte(body), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	fileServer := http.StripPrefix("/static", http.FileServer(http.Dir(dir)))
	handler := middleware.AssetCache(&config.Config{Env: "prod"})(fileServer)

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	req.Header.Set("Range", "bytes=0-4")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("レスポンスボディのクローズのエラー = %v", err)
		}
	})

	if rec.Code != http.StatusPartialContent {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusPartialContent)
	}
	if got := resp.Header.Values("Cache-Control"); len(got) != 1 || got[0] != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %v、期待値 = [public, max-age=31536000, immutable]", got)
	}
	if got := rec.Body.String(); got != body[:5] {
		t.Errorf("本文 = %q、期待値 = %q", got, body[:5])
	}
}
