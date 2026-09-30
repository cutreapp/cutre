package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// findPublishedEvent はユーザー向けの画面で扱うイベントを返す。
//
// イベントが無いときと、公開中でない (アーカイブ・削除した) ときは、ユーザー向けの画面に出さないため
// AppErrCodeResourceNotFound の *model.AppError を返す。
func findPublishedEvent(ctx context.Context, eventRepo *repository.EventRepository, id model.EventID) (*model.Event, error) {
	event, err := eventRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("イベントの取得に失敗: %w", err)
	}
	if event == nil || !event.IsPublished() {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"event_id": id.String()}}
	}

	return event, nil
}
