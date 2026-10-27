package sentry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/getsentry/sentry-go"
	sentryslog "github.com/getsentry/sentry-go/slog"
)

// NewSlogHandler はbaseに流すレコードのうち、ErrorとFatalのものをSentryのイベントとしても送るハンドラーを返す。
//
// baseには logging.NewBaseHandler の戻り値を渡す。
// Sentryへ流してはならないバックグラウンドのロガー (Riverの内部のロガー) はbaseを直接使い、
// 形式と詳細度をそろえたまま、Sentryへのファンアウトの有無だけを切り替える。
//
// Sentry Go 0.48でslogの連携からイベントを作る機能が無くなったため、イベントを作る小さなハンドラーを自前で持つ。
// Sentry Logs APIは使わない。
func NewSlogHandler(base slog.Handler) slog.Handler {
	return newMultiHandler(base, &sentryEventHandler{})
}

// defaultMaxErrorDepth はクライアントが無いときに、ラップされたエラーをたどる深さ。
const defaultMaxErrorDepth = 100

type sentryEventAttr struct {
	groups []string
	attr   slog.Attr
}

// sentryEventHandler はErrorとFatalのレコードだけをSentryのイベントに変える。
// 属性はタグに載せ、error (または err) の属性はイベントの例外にする。
type sentryEventHandler struct {
	attrs  []sentryEventAttr
	groups []string
}

func (h *sentryEventHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level == slog.LevelError || level == sentryslog.LevelFatal
}

func (h *sentryEventHandler) Handle(ctx context.Context, record slog.Record) error {
	// HTTPのリクエストとジョブではcontextに専用のHubが載っており、リクエストIDやジョブの種類のタグが付く。
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}

	event := sentry.NewEvent()
	event.Timestamp = record.Time.UTC()
	event.Level = sentry.LevelError
	if record.Level == sentryslog.LevelFatal {
		event.Level = sentry.LevelFatal
	}
	event.Message = record.Message
	event.Logger = "slog"

	var originalErr error
	for _, a := range h.attrs {
		if err := addSlogAttrToEvent(event, strings.Join(a.groups, "."), a.attr); err != nil && originalErr == nil {
			originalErr = err
		}
	}
	record.Attrs(func(a slog.Attr) bool {
		if err := addSlogAttrToEvent(event, strings.Join(h.groups, "."), a); err != nil && originalErr == nil {
			originalErr = err
		}
		return true
	})

	errorDepth := defaultMaxErrorDepth
	if client := hub.Client(); client != nil {
		errorDepth = client.Options().MaxErrorDepth
	}
	event.SetException(originalErr, errorDepth)
	hub.CaptureEventWithHint(event, &sentry.EventHint{Context: ctx})
	return nil
}

func (h *sentryEventHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := &sentryEventHandler{
		attrs:  make([]sentryEventAttr, 0, len(h.attrs)+len(attrs)),
		groups: append([]string(nil), h.groups...),
	}
	out.attrs = append(out.attrs, h.attrs...)
	for _, a := range attrs {
		out.attrs = append(out.attrs, sentryEventAttr{
			groups: append([]string(nil), h.groups...),
			attr:   a,
		})
	}
	return out
}

func (h *sentryEventHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &sentryEventHandler{
		attrs:  append([]sentryEventAttr(nil), h.attrs...),
		groups: append(append([]string(nil), h.groups...), name),
	}
}

// addSlogAttrToEvent は1つの属性をイベントのタグに加える。
// error の属性はイベントの例外にするため、タグには加えず呼び出し元へ返す。
func addSlogAttrToEvent(event *sentry.Event, group string, attr slog.Attr) error {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return nil
	}

	key := attr.Key
	if group != "" {
		if key != "" {
			key = group + "." + key
		} else {
			key = group
		}
	}

	if attr.Value.Kind() == slog.KindGroup {
		var firstErr error
		for _, child := range attr.Value.Group() {
			if err := addSlogAttrToEvent(event, key, child); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	if (attr.Key == "error" || attr.Key == "err") && attr.Value.Kind() == slog.KindAny {
		if err, ok := attr.Value.Any().(error); ok {
			return err
		}
	}
	if key != "" {
		event.Tags[key] = fmt.Sprint(attr.Value.Any())
	}
	return nil
}

// multiHandler は1つのレコードを複数のハンドラーへ流す。
// 外部の依存 (samber/slog-multiなど) を増やさないよう、必要な分だけを自前で持つ。
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, record.Level) {
			continue
		}
		// 属性を書き換えるハンドラーが後続に影響しないよう、ハンドラーごとに複製して渡す。
		if err := h.Handle(ctx, record.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	// Handleが返せるエラーは1つのため、最初のものだけを返す。
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		out[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: out}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		out[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: out}
}
