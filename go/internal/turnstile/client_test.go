package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cutreapp/cutre/go/internal/testutil"
)

// ハンドラーのテストで差し替えるテストダブルが、インターフェースを満たし続けることを保証する。
var _ Verifier = (*testutil.FakeTurnstileVerifier)(nil)

// blockedTransport はリクエストが送られたらテストを失敗させる。
// siteverifyに問い合わせない経路 (空のシークレットキー・空のトークン) の検証に使う。
type blockedTransport struct {
	t *testing.T
}

func (bt *blockedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	bt.t.Error("siteverifyに問い合わせない経路でリクエストが送られた")
	return nil, errors.New("予期しないリクエスト")
}

// newTestClient はhandlerが応答するsiteverifyに問い合わせるClientを作る。
//
// URLは本番と同じままにし、通信経路だけをメモリ内のテスト用サーバーへ向ける。
// handlerはメソッド・ホスト・パスまで含めて登録するため、Clientが違う宛先へ送ればテストが落ちる。
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST challenges.cloudflare.com/turnstile/v0/siteverify", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			t.Error("HTTPSのリクエストを期待した")
		}
		handler(w, r)
	})
	server := httptest.NewTestServer(t, mux)

	client := NewClient("test-secret-key")
	// Timeoutを保つため、http.Client全体ではなく通信経路だけを差し替える。
	client.httpClient.Transport = server.Client().Transport
	return client
}

func TestClient_Verify_Success(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q、期待値 = %q", got, "application/json")
		}

		// JSONのフィールド名がずれると本番のAPIでしか弾かれないため、ここで捕まえる。
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("リクエストボディのデコードのエラー = %v", err)
		}
		if body["secret"] != "test-secret-key" {
			t.Errorf("secret = %q、期待値 = %q", body["secret"], "test-secret-key")
		}
		if body["response"] != "test-token" {
			t.Errorf("response = %q、期待値 = %q", body["response"], "test-token")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "challenge_ts": "2026-09-21T00:00:00Z", "hostname": "cutre.example.com"}`))
	})

	passed, err := client.Verify(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("Verify()のエラー = %v", err)
	}
	if !passed {
		t.Error("Verify() = false、期待値 = true")
	}
}

func TestClient_Verify_Failure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       string
		wantErrSub string
	}{
		{
			name:       "検証で拒否された",
			status:     http.StatusOK,
			body:       `{"success": false, "error-codes": ["invalid-input-response", "timeout-or-duplicate"]}`,
			wantErrSub: "invalid-input-response timeout-or-duplicate",
		},
		{
			name:       "200以外のステータスが返った",
			status:     http.StatusInternalServerError,
			body:       "Internal Server Error",
			wantErrSub: "ステータスコード: 500",
		},
		{
			name:       "JSONとして読めない応答が返った",
			status:     http.StatusOK,
			body:       `{"success": invalid`,
			wantErrSub: "デコードに失敗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			passed, err := client.Verify(context.Background(), "test-token")
			if passed {
				t.Error("Verify() = true、期待値 = false")
			}
			if err == nil {
				t.Fatal("エラーを期待したが、nilだった")
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("エラー = %q、%qを含むことを期待", err, tt.wantErrSub)
			}
		})
	}
}

func TestClient_Verify_Timeout(t *testing.T) {
	t.Parallel()

	// 合成時間の中で動かし、requestTimeoutを実時間で待たずに検証する。
	synctest.Test(t, func(t *testing.T) {
		// クライアントがタイムアウトするまで応答しない。
		// メモリ内の接続ではクライアントの切断がハンドラーのcontextに届かないため、
		// Verifyが返ったあとにテストから解放し、サーバーの後始末が止まらないようにする。
		release := make(chan struct{})
		client := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
			<-release
		})
		defer close(release)

		start := time.Now()
		passed, err := client.Verify(context.Background(), "test-token")
		if passed {
			t.Error("Verify() = true、期待値 = false")
		}
		if err == nil {
			t.Fatal("エラーを期待したが、nilだった")
		}
		if elapsed := time.Since(start); elapsed != requestTimeout {
			t.Errorf("失敗までの時間 = %v、期待値 = %v", elapsed, requestTimeout)
		}
	})
}

func TestClient_Verify_WithoutRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		secretKey  string
		token      string
		wantPassed bool
	}{
		{
			name:       "シークレットキーが空なら検証せずに通す",
			secretKey:  "",
			token:      "",
			wantPassed: true,
		},
		{
			name:       "トークンが空なら問い合わせずに通さない",
			secretKey:  "test-secret-key",
			token:      "",
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient(tt.secretKey)
			client.httpClient.Transport = &blockedTransport{t: t}

			passed, err := client.Verify(context.Background(), tt.token)
			if err != nil {
				t.Fatalf("Verify()のエラー = %v", err)
			}
			if passed != tt.wantPassed {
				t.Errorf("Verify() = %t、期待値 = %t", passed, tt.wantPassed)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	client := NewClient("test-secret-key")

	// タイムアウトが無いと、siteverifyが応答しないときにハンドラーが止まり続ける。
	if client.httpClient.Timeout != requestTimeout {
		t.Errorf("Timeout = %v、期待値 = %v", client.httpClient.Timeout, requestTimeout)
	}
}
