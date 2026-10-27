// Package logging はアプリケーションのログの出力形式を決める基底のslogハンドラーを提供する。
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// Format はログの出力形式。
type Format string

const (
	// FormatText は人が読みやすいkey=value形式。開発環境の既定。
	FormatText Format = "text"

	// FormatJSON は1行1レコードのJSON形式。収集側が値の型を保ったままフィールドとして取り込める。
	FormatJSON Format = "json"
)

// ParseFormat は設定値からログの出力形式を読み取る。空のときは FormatText を返す。
func ParseFormat(value string) (Format, error) {
	switch Format(value) {
	case "", FormatText:
		return FormatText, nil
	case FormatJSON:
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("ログの出力形式は %q か %q を指定してください: %q", FormatText, FormatJSON, value)
	}
}

// NewBaseHandler はアプリケーションの基底のslogハンドラーを返す。
//
// ログの出力形式とレベルはここだけで決める。
// デフォルトのロガーと、エラートラッカーへ流してはならないバックグラウンドのロガーが
// 同じコンストラクタを使うことで、両者の形式と詳細度がずれないようにする。
//
// レコードのcontextにリクエストIDが載っていれば、request_id として出力に加える。
func NewBaseHandler(w io.Writer, format Format) slog.Handler {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	var h slog.Handler
	if format == FormatJSON {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}

	return &requestIDHandler{inner: h}
}

// requestIDHandler はcontextのリクエストIDをレコードに加えてから内側のハンドラーへ渡す。
// 呼び出し側が毎回 request_id を書かなくても、同じリクエストのログを横断して追えるようにする。
type requestIDHandler struct {
	inner slog.Handler
}

func (h *requestIDHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestIDFromContext(ctx); id != "" {
		// Recordは呼び出し元と属性の領域を共有しうるため、複製してから足す。
		r = r.Clone()
		r.AddAttrs(slog.String("request_id", id))
	}

	return h.inner.Handle(ctx, r)
}

func (h *requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &requestIDHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *requestIDHandler) WithGroup(name string) slog.Handler {
	return &requestIDHandler{inner: h.inner.WithGroup(name)}
}
