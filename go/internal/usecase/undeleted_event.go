package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// findUndeletedEvent は管理画面で扱うイベント (公開中かアーカイブしたもの) を返す。
// 無いときと削除したときは、管理画面にも出さないため AppErrCodeResourceNotFound の *model.AppError を返す。
func findUndeletedEvent(ctx context.Context, eventRepo *repository.EventRepository, id model.EventID) (*model.Event, error) {
	event, err := eventRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("イベントの取得に失敗: %w", err)
	}
	if event == nil || event.IsDeleted() {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"event_id": id.String()}}
	}

	return event, nil
}
