package sentry

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

// recordingHandlerが捕捉した1レコード。Withチェーンで蓄積された属性も
// attrsに平坦化してテストで検証しやすくする。
type recordedLog struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

// 受け取ったslog.Recordをすべて保持するテスト用ハンドラー。
// WithAttrs / WithGroupで派生したハンドラーも同じバッファを共有するため、
// テスト側はルートハンドラーから全レコードを取り出せる。
type recordingHandler struct {
	mu      *sync.Mutex
	records *[]recordedLog
	attrs   []slog.Attr
}

func newRecordingHandler() *recordingHandler {
	return &recordingHandler{
		mu:      &sync.Mutex{},
		records: &[]recordedLog{},
	}
}

func (h *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]string)
	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, recordedLog{
		level: r.Level,
		msg:   r.Message,
		attrs: attrs,
	})
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &recordingHandler{
		mu:      h.mu,
		records: h.records,
	}
	next.attrs = append(next.attrs, h.attrs...)
	next.attrs = append(next.attrs, attrs...)
	return next
}

func (h *recordingHandler) WithGroup(_ string) slog.Handler { return h }

func (h *recordingHandler) Snapshot() []recordedLog {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]recordedLog, len(*h.records))
	copy(out, *h.records)
	return out
}

func TestSlogHandler_ErrorIsCapturedToSentry(t *testing.T) {
	t.Parallel()

	hub, transport := newTestHub(t)
	base := newRecordingHandler()
	logger := slog.New(NewSlogHandler(base))

	ctx := sentry.SetHubOnContext(context.Background(), hub)
	logger.ErrorContext(ctx, "operation failed", "user_id", "u123", "error", errors.New("disk full"))

	hub.Flush(2 * time.Second)
	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("Sentryイベントの件数 = %d、期待値 = 1", len(events))
	}
	got := events[0]
	if got.Message != "operation failed" {
		t.Errorf("event.Message = %q、期待値 = %q", got.Message, "operation failed")
	}
	if got.Tags["user_id"] != "u123" {
		t.Errorf("event.Tags[user_id] = %q、期待値 = %q", got.Tags["user_id"], "u123")
	}
	if len(got.Exception) != 1 || got.Exception[0].Value != "disk full" {
		t.Errorf("例外のvalueの期待値 = %q、実測値 = %+v", "disk full", got.Exception)
	}

	recs := base.Snapshot()
	if len(recs) != 1 {
		t.Fatalf("ベースレコードの件数 = %d、期待値 = 1", len(recs))
	}
	if recs[0].level != slog.LevelError || recs[0].msg != "operation failed" {
		t.Errorf("ベースレコード = %+v、期待値 = Error/operation failed", recs[0])
	}
}

func TestSlogHandler_InfoIsNotCapturedToSentry(t *testing.T) {
	t.Parallel()

	hub, transport := newTestHub(t)
	base := newRecordingHandler()
	logger := slog.New(NewSlogHandler(base))

	ctx := sentry.SetHubOnContext(context.Background(), hub)
	logger.InfoContext(ctx, "request received", "path", "/")

	hub.Flush(2 * time.Second)
	if got := len(transport.Events()); got != 0 {
		t.Errorf("InfoレベルのSentryイベントの件数 = %d、期待値 = 0", got)
	}

	recs := base.Snapshot()
	if len(recs) != 1 {
		t.Fatalf("ベースレコードの件数 = %d、期待値 = 1", len(recs))
	}
	if recs[0].level != slog.LevelInfo || recs[0].msg != "request received" {
		t.Errorf("ベースレコード = %+v、期待値 = Info/request received", recs[0])
	}
}

func TestSlogHandler_BaseHandlerReceivesAllLevels(t *testing.T) {
	t.Parallel()

	// baseハンドラーは構造化ログの出力先なので、Sentryが取り上げるか
	// どうかに関わらず全レベルが届く必要がある。これにより、開発者は標準
	// エラー出力でInfo/Warnを引き続き追える一方、SentryはError以上に
	// 集中できる。
	hub, transport := newTestHub(t)
	base := newRecordingHandler()
	logger := slog.New(NewSlogHandler(base))

	ctx := sentry.SetHubOnContext(context.Background(), hub)
	logger.DebugContext(ctx, "debug msg")
	logger.InfoContext(ctx, "info msg")
	logger.WarnContext(ctx, "warn msg")
	logger.ErrorContext(ctx, "error msg")

	hub.Flush(2 * time.Second)
	if got := len(transport.Events()); got != 1 {
		t.Errorf("Sentryイベントの件数 = %d、期待値 = 1 (Errorのみ)", got)
	}

	recs := base.Snapshot()
	want := []struct {
		level slog.Level
		msg   string
	}{
		{slog.LevelDebug, "debug msg"},
		{slog.LevelInfo, "info msg"},
		{slog.LevelWarn, "warn msg"},
		{slog.LevelError, "error msg"},
	}
	if len(recs) != len(want) {
		t.Fatalf("ベースレコードの件数の期待値 = %d、実測値 = %d", len(want), len(recs))
	}
	for i, w := range want {
		if recs[i].level != w.level || recs[i].msg != w.msg {
			t.Errorf("ベースレコード[%d] = (%v, %q)、期待値 = (%v, %q)", i, recs[i].level, recs[i].msg, w.level, w.msg)
		}
	}
}
func TestSlogHandler_MasksSensitiveTagsEndToEnd(t *testing.T) {
	t.Parallel()

	// End-to-Endの確認: slog属性として載せたメールアドレスはbeforeSend
	// でマスクされてからtransportに届くこと。baseハンドラーには元の値が
	// 届き、標準エラー出力でのデバッグには引き続き使えること。
	hub, transport := newTestHubWithBeforeSend(t)
	base := newRecordingHandler()
	logger := slog.New(NewSlogHandler(base))

	ctx := sentry.SetHubOnContext(context.Background(), hub)
	logger.ErrorContext(ctx, "email send failure",
		"email", "user@example.com",
		"user_id", "u123",
		"error", errors.New("send failure"),
	)

	hub.Flush(2 * time.Second)
	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("Sentryイベントの件数 = %d、期待値 = 1", len(events))
	}
	tags := events[0].Tags
	if tags["email"] != "[FILTERED]" {
		t.Errorf("event.Tags[email] = %q、期待値 = %q", tags["email"], "[FILTERED]")
	}
	if tags["user_id"] != "u123" {
		t.Errorf("event.Tags[user_id] = %q、期待値 = %q", tags["user_id"], "u123")
	}

	recs := base.Snapshot()
	if len(recs) != 1 {
		t.Fatalf("ベースレコードの件数 = %d、期待値 = 1", len(recs))
	}
	if recs[0].attrs["email"] != "user@example.com" {
		t.Errorf("ベースレコードのattrs[email] = %q、期待値 = %q", recs[0].attrs["email"], "user@example.com")
	}
}

func TestSlogHandler_WithAttrs_PropagatesToBothHandlers(t *testing.T) {
	t.Parallel()

	// Logger.Withで追加した属性はbaseとSentryの両方に届く必要がある。
	// さもないとリクエスト単位の情報 (例: request_id) が片方に欠落し、
	// 標準エラー出力のログとSentryイベントを突き合わせられなくなる。
	hub, transport := newTestHub(t)
	base := newRecordingHandler()
	logger := slog.New(NewSlogHandler(base)).With("request_id", "r123")

	ctx := sentry.SetHubOnContext(context.Background(), hub)
	logger.ErrorContext(ctx, "boom", "user_id", "u456")

	hub.Flush(2 * time.Second)
	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("Sentryイベントの件数 = %d、期待値 = 1", len(events))
	}
	tags := events[0].Tags
	if tags["request_id"] != "r123" {
		t.Errorf("event.Tags[request_id] = %q、期待値 = %q", tags["request_id"], "r123")
	}
	if tags["user_id"] != "u456" {
		t.Errorf("event.Tags[user_id] = %q、期待値 = %q", tags["user_id"], "u456")
	}

	recs := base.Snapshot()
	if len(recs) != 1 {
		t.Fatalf("ベースレコードの件数 = %d、期待値 = 1", len(recs))
	}
	if recs[0].attrs["request_id"] != "r123" {
		t.Errorf("ベースレコードのattrs[request_id] = %q、期待値 = %q", recs[0].attrs["request_id"], "r123")
	}
	if recs[0].attrs["user_id"] != "u456" {
		t.Errorf("ベースレコードのattrs[user_id] = %q、期待値 = %q", recs[0].attrs["user_id"], "u456")
	}
}
