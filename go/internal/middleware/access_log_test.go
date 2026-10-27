package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/logging"
	"github.com/cutreapp/cutre/go/internal/middleware"
)

// captureAccessLog はデフォルトのロガーを基底のJSONハンドラーへ差し替え、出力先を返す。
// デフォルトのロガーはプロセス全体で共有するため、これを使うテストは t.Parallel() を呼ばない。
func captureAccessLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(logging.NewBaseHandler(&buf, logging.FormatJSON)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &buf
}

// decodeAccessLogs は出力されたJSONのログを1行ずつ読み取る。
func decodeAccessLogs(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	dec := json.NewDecoder(buf)
	for dec.More() {
		var record map[string]any
		if err := dec.Decode(&record); err != nil {
			t.Fatalf("ログのデコードのエラー = %v", err)
		}
		records = append(records, record)
	}

	return records
}

// TestAccessLog は、アクセスログにルートパターン・ステータス・リクエストID・Cf-Rayが載り、
// パスそのものは載らないことを検証する。
func TestAccessLog(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantRoute  string
		wantStatus float64
	}{
		{name: "ステータスを書いたルート", target: "/password_resets/secret-token", wantRoute: "/password_resets/{token}", wantStatus: http.StatusSeeOther},
		{name: "本文だけを書いたルート", target: "/body", wantRoute: "/body", wantStatus: http.StatusOK},
		{name: "何も書かないルート", target: "/empty", wantRoute: "/empty", wantStatus: http.StatusOK},
		{name: "一致するルートが無い", target: "/missing", wantRoute: "", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureAccessLog(t)

			r := chi.NewRouter()
			r.Use(middleware.RequestID, middleware.AccessLog)
			r.Get("/password_resets/{token}", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/sign_in", http.StatusSeeOther)
			})
			r.Get("/body", func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("ok")); err != nil {
					t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
				}
			})
			r.Get("/empty", func(http.ResponseWriter, *http.Request) {})

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req.Header.Set("Cf-Ray", "8f1e2d3c4b5a6978-NRT")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			records := decodeAccessLogs(t, buf)
			if len(records) != 1 {
				t.Fatalf("ログの件数 = %d、期待値 = 1", len(records))
			}
			record := records[0]

			if got := record["method"]; got != http.MethodGet {
				t.Errorf("method = %v、期待値 = %s", got, http.MethodGet)
			}
			if got := record["route"]; got != tt.wantRoute {
				t.Errorf("route = %v、期待値 = %q", got, tt.wantRoute)
			}
			if got := record["status"]; got != tt.wantStatus {
				t.Errorf("status = %v、期待値 = %v", got, tt.wantStatus)
			}
			if got := record["cf_ray"]; got != "8f1e2d3c4b5a6978-NRT" {
				t.Errorf("cf_ray = %v、期待値 = 8f1e2d3c4b5a6978-NRT", got)
			}
			if _, ok := record["duration_ms"].(float64); !ok {
				t.Errorf("duration_ms = %#v、期待値 = 数値", record["duration_ms"])
			}
			if got, want := record["request_id"], rec.Header().Get("X-Request-Id"); got != want || want == "" {
				t.Errorf("request_id = %v、期待値 = レスポンスの X-Request-Id (%q)", got, want)
			}
			if bytes.Contains(buf.Bytes(), []byte("secret-token")) {
				t.Errorf("パスに含まれるトークンがログに出力された: %v", record)
			}
		})
	}
}

// TestAccessLog_Panic は、Recovererより外側に置いたとき、panicから返す500を記録することを検証する。
func TestAccessLog_Panic(t *testing.T) {
	buf := captureAccessLog(t)

	r := chi.NewRouter()
	// Recovererが出力するスタックトレースでテストの出力を汚さないよう、panicを500へ変える最小のハンドラーで代用する。
	recoverer := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recover() != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
	r.Use(middleware.AccessLog, recoverer)
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("テスト") })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))

	records := decodeAccessLogs(t, buf)
	if len(records) != 1 {
		t.Fatalf("ログの件数 = %d、期待値 = 1", len(records))
	}
	if got := records[0]["status"]; got != float64(http.StatusInternalServerError) {
		t.Errorf("status = %v、期待値 = %d", got, http.StatusInternalServerError)
	}
}

// TestAccessLog_Flush は、Flushで確定したステータスを、その後のエラー応答で上書きしないことを検証する。
// デフォルトのロガーを差し替えるため、並行実行しない。
func TestAccessLog_Flush(t *testing.T) {
	tests := []struct {
		name            string
		initialStatus   int
		panicAfterFlush bool
	}{
		{name: "暗黙の200のあとで500を指定する"},
		{name: "暗黙の200のあとでpanicから回復する", panicAfterFlush: true},
		{name: "明示した202を維持する", initialStatus: http.StatusAccepted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureAccessLog(t)
			handler := middleware.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// Recovererと同じく、panicを500へ変換する。
				defer func() {
					if recover() != nil {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}()
				if tt.initialStatus != 0 {
					w.WriteHeader(tt.initialStatus)
				}
				if err := http.NewResponseController(w).Flush(); err != nil {
					t.Fatalf("Flush()のエラー = %v", err)
				}
				if tt.panicAfterFlush {
					panic("テスト")
				}
				w.WriteHeader(http.StatusInternalServerError)
			}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			if !rec.Flushed {
				t.Error("下層のResponseWriterがFlushされなかった")
			}
			wantStatus := tt.initialStatus
			if wantStatus == 0 {
				wantStatus = http.StatusOK
			}
			if rec.Code != wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, wantStatus)
			}
			records := decodeAccessLogs(t, buf)
			if len(records) != 1 {
				t.Fatalf("ログの件数 = %d、期待値 = 1", len(records))
			}
			if got := records[0]["status"]; got != float64(rec.Code) {
				t.Errorf("ログのstatus = %v、期待値 = 実レスポンスの%d", got, rec.Code)
			}
		})
	}
}

// TestAccessLog_FlushUnsupported は、Flush非対応ならステータスが未確定のままであることを検証する。
// デフォルトのロガーを差し替えるため、並行実行しない。
func TestAccessLog_FlushUnsupported(t *testing.T) {
	buf := captureAccessLog(t)
	handler := middleware.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); !errors.Is(err, http.ErrNotSupported) {
			t.Errorf("Flush()のエラー = %v、期待値 = http.ErrNotSupported", err)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	rec := httptest.NewRecorder()
	// ResponseWriterのメソッドだけを公開し、RecorderのFlushを隠す。
	handler.ServeHTTP(struct{ http.ResponseWriter }{rec}, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = 500", rec.Code)
	}
	records := decodeAccessLogs(t, buf)
	if len(records) != 1 {
		t.Fatalf("ログの件数 = %d、期待値 = 1", len(records))
	}
	if got := records[0]["status"]; got != float64(rec.Code) {
		t.Errorf("ログのstatus = %v、期待値 = 実レスポンスの%d", got, rec.Code)
	}
}

// failingFlushWriter は、net/httpと同じくヘッダーの送出を確定してから書き出しに失敗するResponseWriter。
// クライアントの切断などでFlushが途中で失敗した状況を再現する。
type failingFlushWriter struct {
	*httptest.ResponseRecorder
}

func (w failingFlushWriter) FlushError() error {
	w.WriteHeader(http.StatusOK)

	return errors.New("書き出しに失敗しました")
}

// TestAccessLog_FlushFailed は、Flushが送出の途中で失敗しても、確定した200を記録することを検証する。
// デフォルトのロガーを差し替えるため、並行実行しない。
func TestAccessLog_FlushFailed(t *testing.T) {
	buf := captureAccessLog(t)
	handler := middleware.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err == nil || errors.Is(err, http.ErrNotSupported) {
			t.Errorf("Flush()のエラー = %v、期待値 = 書き出しのエラー", err)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(failingFlushWriter{rec}, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = 200", rec.Code)
	}
	records := decodeAccessLogs(t, buf)
	if len(records) != 1 {
		t.Fatalf("ログの件数 = %d、期待値 = 1", len(records))
	}
	if got := records[0]["status"]; got != float64(rec.Code) {
		t.Errorf("ログのstatus = %v、期待値 = 実レスポンスの%d", got, rec.Code)
	}
}
