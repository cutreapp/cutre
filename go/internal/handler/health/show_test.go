package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/handler/health"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestShow は、データベースに届くときは200を返し、届かないときは503を返すことを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupRepo      func(t *testing.T) *repository.HealthRepository
		wantCode       int
		wantStatus     string
		wantRetryAfter string
	}{
		{
			name: "データベースに届くときは200を返す",
			setupRepo: func(t *testing.T) *repository.HealthRepository {
				db, tx := testutil.SetupTx(t)
				return repository.NewHealthRepository(db).WithTx(tx)
			},
			wantCode:   http.StatusOK,
			wantStatus: "ok",
		},
		{
			name: "データベースに届かないときは503を返す",
			setupRepo: func(t *testing.T) *repository.HealthRepository {
				return repository.NewHealthRepository(testutil.OpenUnreachableDB(t))
			},
			wantCode:       http.StatusServiceUnavailable,
			wantStatus:     "unavailable",
			wantRetryAfter: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := health.NewHandler(usecase.NewCheckHealthUsecase(tt.setupRepo(t)))

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantCode)
			}

			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q、期待値 = %q", got, "application/json")
			}

			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q、期待値 = no-store", got)
			}
			if got := rec.Header().Get("Retry-After"); got != tt.wantRetryAfter {
				t.Errorf("Retry-After = %q、期待値 = %q", got, tt.wantRetryAfter)
			}

			// 応答には状態だけを載せ、接続の失敗の原因 (接続先など) は出さない。
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("レスポンスボディのデコードのエラー = %v", err)
			}
			want := map[string]string{"status": tt.wantStatus, "database": tt.wantStatus}
			if len(body) != len(want) || body["status"] != want["status"] || body["database"] != want["database"] {
				t.Errorf("ボディ = %v、期待値 = %v", body, want)
			}
		})
	}
}
