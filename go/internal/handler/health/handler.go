// Package health はヘルスチェックエンドポイントのハンドラーを提供する。
package health

import "github.com/cutreapp/cutre/go/internal/usecase"

// Handler はヘルスチェックエンドポイントのHTTPハンドラー。
type Handler struct {
	checkHealthUC *usecase.CheckHealthUsecase
}

// NewHandler は新しいHandlerを作成する。
func NewHandler(checkHealthUC *usecase.CheckHealthUsecase) *Handler {
	return &Handler{checkHealthUC: checkHealthUC}
}
