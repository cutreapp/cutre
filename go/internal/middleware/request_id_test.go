package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/logging"
	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestRequestID は、採番したIDをレスポンスとcontextの両方に載せ、受信した X-Request-Id を使わないことを検証する。
func TestRequestID(t *testing.T) {
	t.Parallel()

	var ctxID string
	handler := middleware.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctxID = logging.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "client-supplied")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("X-Request-Id")
	if got == "" || got == "client-supplied" {
		t.Errorf("X-Request-Id = %q、期待値 = 自前で採番した値", got)
	}
	if ctxID != got {
		t.Errorf("contextのリクエストID = %q、期待値 = レスポンスの X-Request-Id (%q)", ctxID, got)
	}
}

// TestRequestID_Unique は、リクエストごとに異なるIDを採番することを検証する。
func TestRequestID_Unique(t *testing.T) {
	t.Parallel()

	handler := middleware.RequestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	ids := make(map[string]struct{})
	for range 3 {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		ids[rec.Header().Get("X-Request-Id")] = struct{}{}
	}

	if len(ids) != 3 {
		t.Errorf("異なるIDの数 = %d、期待値 = 3", len(ids))
	}
}
