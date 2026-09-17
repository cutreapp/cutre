// Package health はヘルスチェックエンドポイントのハンドラーを提供する。
package health

// Handler はヘルスチェックエンドポイントのHTTPハンドラー。
type Handler struct{}

// NewHandler は新しいHandlerを作成する。
func NewHandler() *Handler {
	return &Handler{}
}
