package sentry

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

// captureTransport はSentryのクライアントがネットワークへ送るはずのイベントをすべて集める。
// トランザクションの終了とpanicの捕捉が別のゴルーチンから送ることがあるため、排他制御で守る。
type captureTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *captureTransport) Configure(_ sentry.ClientOptions)        {}
func (t *captureTransport) Flush(_ time.Duration) bool              { return true }
func (t *captureTransport) FlushWithContext(_ context.Context) bool { return true }
func (t *captureTransport) Close()                                  {}
func (t *captureTransport) SendEventWithContext(_ context.Context, e *sentry.Event) {
	t.SendEvent(e)
}

func (t *captureTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *captureTransport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*sentry.Event, len(t.events))
	copy(out, t.events)
	return out
}

// newTestHub はテストごとに独立したHubとcaptureTransportを作る。グローバルのHubには触らない。
func newTestHub(t *testing.T) (*sentry.Hub, *captureTransport) {
	t.Helper()
	return newTestHubWithOptions(t, sentry.ClientOptions{})
}

// newTestHubWithBeforeSend は本番と同じbeforeSendを組み込んだHubを作る。
// slog → イベントのハンドラー → beforeSend の流れを通しで確かめるために使う。
func newTestHubWithBeforeSend(t *testing.T) (*sentry.Hub, *captureTransport) {
	t.Helper()
	return newTestHubWithOptions(t, sentry.ClientOptions{BeforeSend: beforeSend})
}

func newTestHubWithOptions(t *testing.T, opts sentry.ClientOptions) (*sentry.Hub, *captureTransport) {
	t.Helper()
	transport := &captureTransport{}
	opts.Dsn = "https://public@example.com/1"
	opts.Transport = transport
	client, err := sentry.NewClient(opts)
	if err != nil {
		t.Fatalf("sentry.NewClient()のエラー = %v", err)
	}
	return sentry.NewHub(client, sentry.NewScope()), transport
}
