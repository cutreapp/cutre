package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/metrics"
	"github.com/cutreapp/cutre/go/internal/middleware"
)

// scrapeMetrics はレジストリのメトリクスをPrometheusのテキスト形式で取り出す。
func scrapeMetrics(t *testing.T, reg *metrics.Registry) string {
	t.Helper()

	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, metrics.Path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("メトリクスのステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("メトリクスの読み取りのエラー = %v", err)
	}

	return string(body)
}

// TestMetrics は、リクエストをルートパターン・メソッド・ステータス別に数え、パスそのものはラベルに使わないことを検証する。
func TestMetrics(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	r := chi.NewRouter()
	r.Use(middleware.Metrics(reg.NewHTTP()))
	r.Get("/password_resets/{token}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sign_in", http.StatusSeeOther)
	})
	r.Get("/empty", func(http.ResponseWriter, *http.Request) {})

	for _, target := range []string{"/password_resets/secret-token", "/password_resets/another-token", "/empty", "/missing"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	}

	got := scrapeMetrics(t, reg)
	for _, want := range []string{
		`http_request_duration_seconds_count{method="GET",route="/password_resets/{token}",status="303"} 2`,
		`http_request_duration_seconds_count{method="GET",route="/empty",status="200"} 1`,
		`http_request_duration_seconds_count{method="GET",route="",status="404"} 1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("メトリクスに %s が含まれない\n%s", want, got)
		}
	}
	if strings.Contains(got, "secret-token") {
		t.Error("パスに含まれるトークンがメトリクスに出力された")
	}
}

// TestMetrics_Panic は、Recovererより外側に置いたとき、panicから返す500を数えることを検証する。
func TestMetrics_Panic(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
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
	r.Use(middleware.Metrics(reg.NewHTTP()), recoverer)
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("テスト") })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))

	want := `http_request_duration_seconds_count{method="GET",route="/panic",status="500"} 1`
	if got := scrapeMetrics(t, reg); !strings.Contains(got, want) {
		t.Errorf("メトリクスに %s が含まれない\n%s", want, got)
	}
}
