package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
)

// sentryCaptureTransport はSentryのクライアントがネットワークへ送るはずのイベントをすべて集める。
// トランザクションの終了とpanicの捕捉が別のゴルーチンから送ることがあるため、排他制御で守る。
type sentryCaptureTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *sentryCaptureTransport) Configure(_ sentry.ClientOptions)        {}
func (t *sentryCaptureTransport) Flush(_ time.Duration) bool              { return true }
func (t *sentryCaptureTransport) FlushWithContext(_ context.Context) bool { return true }
func (t *sentryCaptureTransport) Close()                                  {}
func (t *sentryCaptureTransport) SendEventWithContext(_ context.Context, e *sentry.Event) {
	t.SendEvent(e)
}

func (t *sentryCaptureTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

// errorEvents はトレース以外のイベントを返す。
func (t *sentryCaptureTransport) errorEvents() []*sentry.Event {
	return t.filter(func(e *sentry.Event) bool { return e.Type != "transaction" })
}

// transactionEvents はトレースのイベントを返す。
func (t *sentryCaptureTransport) transactionEvents() []*sentry.Event {
	return t.filter(func(e *sentry.Event) bool { return e.Type == "transaction" })
}

func (t *sentryCaptureTransport) filter(match func(*sentry.Event) bool) []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []*sentry.Event
	for _, e := range t.events {
		if match(e) {
			out = append(out, e)
		}
	}
	return out
}

// newSentryTestHub はテストごとに独立したHubとTransportを作る。グローバルのHubには触らない。
func newSentryTestHub(t *testing.T) (*sentry.Hub, *sentryCaptureTransport) {
	t.Helper()
	transport := &sentryCaptureTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              "https://public@example.com/1",
		Transport:        transport,
		EnableTracing:    true,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		t.Fatalf("sentry.NewClient()のエラー = %v", err)
	}
	return sentry.NewHub(client, sentry.NewScope()), transport
}

// attachSentryHub はテスト用のHubをリクエストのcontextに載せる。
// sentryhttpはcontextにHubがあればそれを使うため、グローバルのHubを複製させずに済む。
func attachSentryHub(hub *sentry.Hub) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(sentry.SetHubOnContext(r.Context(), hub)))
		})
	}
}

// newSentryRouter はserve.goと同じ順 (RequestID → Recoverer → sentryhttp → SentryTransaction → SentryRequestID) のルーターを作る。
func newSentryRouter(hub *sentry.Hub, register func(chi.Router)) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Use(attachSentryHub(hub))
	r.Use(sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle)
	r.Use(middleware.SentryTransaction)
	r.Use(middleware.SentryRequestID)
	register(r)
	return r
}

// TestSentryTransaction_PanicEvent は、panicのイベントとトレースがルートパターンで表されることを検証する。
func TestSentryTransaction_PanicEvent(t *testing.T) {
	t.Parallel()

	hub, transport := newSentryTestHub(t)
	router := newSentryRouter(hub, func(r chi.Router) {
		r.Get("/invitations/{token}", func(http.ResponseWriter, *http.Request) {
			panic(errors.New("boom"))
		})
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invitations/secret-token", nil))

	// Recovererが再panicを受け止めて500を返す。
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}

	hub.Flush(2 * time.Second)

	errEvents := transport.errorEvents()
	if len(errEvents) != 1 {
		t.Fatalf("エラーのイベントの件数 = %d、期待値 = 1", len(errEvents))
	}
	if got, want := errEvents[0].Transaction, "GET /invitations/{token}"; got != want {
		t.Errorf("エラーのイベントのTransaction = %q、期待値 = %q", got, want)
	}
	if got, want := errEvents[0].Request.URL, "http://example.com/invitations/%7Btoken%7D"; got != want {
		t.Errorf("エラーのイベントのRequest.URL = %q、期待値 = %q", got, want)
	}

	txEvents := transport.transactionEvents()
	if len(txEvents) != 1 {
		t.Fatalf("トレースの件数 = %d、期待値 = 1", len(txEvents))
	}
	if got, want := txEvents[0].Transaction, "GET /invitations/{token}"; got != want {
		t.Errorf("トレースのTransaction = %q、期待値 = %q", got, want)
	}
	if got := txEvents[0].TransactionInfo; got == nil || got.Source != sentry.SourceRoute {
		t.Errorf("トレースのTransactionInfo = %+v、期待値のSource = %q", got, sentry.SourceRoute)
	}
	if got, want := txEvents[0].Request.URL, "http://example.com/invitations/%7Btoken%7D"; got != want {
		t.Errorf("トレースのRequest.URL = %q、期待値 = %q", got, want)
	}
	trace := txEvents[0].Contexts["trace"]
	if got := trace["status"]; got != sentry.SpanStatusInternalError {
		t.Errorf("トレースの状態 = %v、期待値 = internal_error", got)
	}
	if got := trace["data"].(map[string]any)["http.response.status_code"]; got != http.StatusInternalServerError {
		t.Errorf("トレースのHTTPステータス = %v、期待値 = 500", got)
	}
}

// TestSentryTransaction_CapturedError は、ハンドラーの中で送ったイベントにもルートパターンとリクエストIDが載ることを検証する。
func TestSentryTransaction_CapturedError(t *testing.T) {
	t.Parallel()

	hub, transport := newSentryTestHub(t)
	router := newSentryRouter(hub, func(r chi.Router) {
		r.Get("/@{atname}", func(w http.ResponseWriter, r *http.Request) {
			sentry.GetHubFromContext(r.Context()).CaptureException(errors.New("lookup failed"))
			w.WriteHeader(http.StatusOK)
		})
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/@alice", nil))

	hub.Flush(2 * time.Second)

	errEvents := transport.errorEvents()
	if len(errEvents) != 1 {
		t.Fatalf("エラーのイベントの件数 = %d、期待値 = 1", len(errEvents))
	}
	if got, want := errEvents[0].Transaction, "GET /@{atname}"; got != want {
		t.Errorf("Transaction = %q、期待値 = %q", got, want)
	}
	if got, want := errEvents[0].Tags["request_id"], rec.Header().Get("X-Request-Id"); got == "" || got != want {
		t.Errorf("タグrequest_id = %q、期待値 = レスポンスの X-Request-Id (%q)", got, want)
	}
}

// TestSentryTransaction_UnmatchedRoute は、一致するルートが無いときにパスを送らないことを検証する。
func TestSentryTransaction_UnmatchedRoute(t *testing.T) {
	t.Parallel()

	hub, transport := newSentryTestHub(t)
	router := newSentryRouter(hub, func(r chi.Router) {
		// ルートが1つも無いchiのルーターはミドルウェアを通さないため、一致しないルートを1つ置く。
		r.Get("/known", func(http.ResponseWriter, *http.Request) {})
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			sentry.GetHubFromContext(r.Context()).CaptureMessage("not found")
			// SDKは既定で404のトレースを送らないため、トレースの名前を確かめられるよう200で応える。
			w.WriteHeader(http.StatusOK)
		})
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invitations/secret-token/extra", nil))

	hub.Flush(2 * time.Second)

	errEvents := transport.errorEvents()
	if len(errEvents) != 1 {
		t.Fatalf("エラーのイベントの件数 = %d、期待値 = 1", len(errEvents))
	}
	if got, want := errEvents[0].Request.URL, "http://example.com"; got != want {
		t.Errorf("エラーのイベントのRequest.URL = %q、期待値 = %q", got, want)
	}

	txEvents := transport.transactionEvents()
	if len(txEvents) != 1 {
		t.Fatalf("トレースの件数 = %d、期待値 = 1", len(txEvents))
	}
	if got, want := txEvents[0].Transaction, "GET"; got != want {
		t.Errorf("トレースのTransaction = %q、期待値 = %q", got, want)
	}
}

// TestSentryTransaction_NoChiContext は、chiを通らないリクエストでも何もせずに後続へ渡すことを検証する。
func TestSentryTransaction_NoChiContext(t *testing.T) {
	t.Parallel()

	hub, transport := newSentryTestHub(t)

	called := false
	handler := middleware.SentryTransaction(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	req = req.WithContext(sentry.SetHubOnContext(req.Context(), hub))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("後続のハンドラーが呼ばれなかった")
	}

	hub.Flush(100 * time.Millisecond)
	if got := len(transport.errorEvents()) + len(transport.transactionEvents()); got != 0 {
		t.Errorf("イベントの件数 = %d、期待値 = 0", got)
	}
}

// TestSentryUserContext は、ログイン中のユーザーのIDだけをSentryのスコープに載せることを検証する。
func TestSentryUserContext(t *testing.T) {
	t.Parallel()

	userID := model.UserID(uuid.New())
	tests := []struct {
		name string
		user *model.User
		want string
	}{
		{
			name: "ログイン中はIDだけを載せる",
			user: &model.User{ID: userID, Email: "user@example.com", Atname: "alice"},
			want: userID.String(),
		},
		{name: "未ログインなら載せない", user: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hub, transport := newSentryTestHub(t)
			handler := middleware.SentryUserContext(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				sentry.GetHubFromContext(r.Context()).CaptureMessage("test")
			}))

			ctx := sentry.SetHubOnContext(context.Background(), hub)
			if tt.user != nil {
				ctx = middleware.SetUserToContext(ctx, tt.user)
			}
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
			handler.ServeHTTP(httptest.NewRecorder(), req)

			hub.Flush(2 * time.Second)

			events := transport.errorEvents()
			if len(events) != 1 {
				t.Fatalf("イベントの件数 = %d、期待値 = 1", len(events))
			}
			got := events[0].User
			if got.ID != tt.want {
				t.Errorf("User.ID = %q、期待値 = %q", got.ID, tt.want)
			}
			if got.Email != "" || got.Username != "" {
				t.Errorf("User = %+v、期待値 = IDだけ", got)
			}
		})
	}
}

// TestSentryUserContext_WithoutHub は、contextにHubが無くても後続へ渡すことを検証する。
func TestSentryUserContext_WithoutHub(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.SentryUserContext(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	ctx := middleware.SetUserToContext(context.Background(), &model.User{ID: model.UserID(uuid.New())})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))

	if !called {
		t.Error("後続のハンドラーが呼ばれなかった")
	}
}

// TestSentryTransaction_AbortHandler は接続の中断を500や成功のトレースとして送らないことを検証する。
func TestSentryTransaction_AbortHandler(t *testing.T) {
	t.Parallel()
	hub, transport := newSentryTestHub(t)
	router := newSentryRouter(hub, func(r chi.Router) {
		r.Get("/abort", func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
	})
	func() {
		defer func() {
			if got := recover(); got != http.ErrAbortHandler {
				t.Errorf("再送出されたpanic = %v、期待値 = ErrAbortHandler", got)
			}
		}()
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/abort", nil))
		t.Error("接続の中断が外側へ伝わらなかった")
	}()
	if got := len(transport.transactionEvents()); got != 0 {
		t.Errorf("中断のトレースの件数 = %d、期待値 = 0", got)
	}
}

// TestSentryTransaction_PanicAfterWrite はpanic前に送ったステータスをトレースでも保持することを検証する。
func TestSentryTransaction_PanicAfterWrite(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		status int
		flush  bool
	}{
		{name: "明示したステータス", status: http.StatusAccepted},
		{name: "Flushの暗黙の200", status: http.StatusOK, flush: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hub, transport := newSentryTestHub(t)
			router := newSentryRouter(hub, func(r chi.Router) {
				r.Get("/partial", func(w http.ResponseWriter, _ *http.Request) {
					if tt.flush {
						if err := http.NewResponseController(w).Flush(); err != nil {
							t.Fatalf("Flush()のエラー = %v", err)
						}
					} else {
						w.WriteHeader(tt.status)
					}
					panic("応答後の失敗")
				})
			})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/partial", nil))
			events := transport.transactionEvents()
			if len(events) != 1 {
				t.Fatalf("トレースの件数 = %d、期待値 = 1", len(events))
			}
			trace := events[0].Contexts["trace"]
			if got := trace["data"].(map[string]any)["http.response.status_code"]; got != rec.Code || rec.Code != tt.status {
				t.Errorf("トレースのHTTPステータス = %v、実際の応答 = %d、期待値 = %d", got, rec.Code, tt.status)
			}
			if got := trace["status"]; got != sentry.SpanStatusInternalError {
				t.Errorf("トレースの状態 = %v、期待値 = internal_error", got)
			}
		})
	}
}
