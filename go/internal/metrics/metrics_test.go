package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/metrics"
)

// scrape はハンドラーへリクエストを送り、ステータスコードと本文を返す。
func scrape(t *testing.T, reg *metrics.Registry, method, target string) (int, string) {
	t.Helper()

	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(method, target, nil))

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("本文の読み取りのエラー = %v", err)
	}

	return rec.Code, string(body)
}

// TestRegistry_Handler は、/metrics でGoのランタイムとプロセスのメトリクスを返し、それ以外のパスは返さないことを検証する。
func TestRegistry_Handler(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	status, body := scrape(t, reg, http.MethodGet, metrics.Path)
	if status != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", status, http.StatusOK)
	}
	for _, want := range []string{"go_goroutines ", "go_memstats_heap_alloc_bytes ", "process_resident_memory_bytes "} {
		if !strings.Contains(body, want) {
			t.Errorf("メトリクスに %q が含まれない", want)
		}
	}

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
	}{
		{name: "ほかのパス", method: http.MethodGet, target: "/", wantStatus: http.StatusNotFound},
		{name: "GET以外のメソッド", method: http.MethodPost, target: metrics.Path, wantStatus: http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if status, _ := scrape(t, reg, tt.method, tt.target); status != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", status, tt.wantStatus)
			}
		})
	}
}
