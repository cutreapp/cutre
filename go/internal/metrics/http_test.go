package metrics_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/metrics"
)

// TestHTTP_Observe は、件数と所要時間をラベル別に記録し、未知のメソッドを1つのラベルにまとめることを検証する。
func TestHTTP_Observe(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	h := reg.NewHTTP()

	h.Observe("/items/{id}", http.MethodGet, http.StatusOK, 30*time.Millisecond)
	h.Observe("/items/{id}", http.MethodGet, http.StatusOK, 2*time.Second)
	h.Observe("/items/{id}", "PROPFIND", http.StatusMethodNotAllowed, time.Millisecond)
	h.Observe("/items/{id}", "X-RANDOM-1", http.StatusMethodNotAllowed, time.Millisecond)

	_, body := scrape(t, reg, http.MethodGet, metrics.Path)
	for _, want := range []string{
		`http_request_duration_seconds_count{method="GET",route="/items/{id}",status="200"} 2`,
		`http_request_duration_seconds_sum{method="GET",route="/items/{id}",status="200"} 2.03`,
		`http_request_duration_seconds_bucket{method="GET",route="/items/{id}",status="200",le="0.05"} 1`,
		`http_request_duration_seconds_count{method="OTHER",route="/items/{id}",status="405"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("メトリクスに %s が含まれない\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"PROPFIND", "X-RANDOM-1"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("未知のメソッド %s がラベルに使われた", unwanted)
		}
	}
}
