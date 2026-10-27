package logging

import (
	"context"
	"crypto/rand"
)

type requestIDKey struct{}

// NewRequestID はリクエストIDを新しく採番する。
// 128ビットの乱数をbase32で表した26文字で、プロセスやホストをまたいでも衝突しない。
func NewRequestID() string {
	return rand.Text()
}

// WithRequestID はリクエストIDを載せたcontextを返す。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext はcontextに載っているリクエストIDを返す。載っていなければ空文字列を返す。
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
