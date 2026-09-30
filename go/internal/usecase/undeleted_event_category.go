package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// findUndeletedEventCategory は管理画面で扱うカテゴリーと、そのカテゴリーのイベントを返す。
//
// カテゴリーが無いときと、カテゴリーかイベントを削除したときは、管理画面にも出さないため
// AppErrCodeResourceNotFound の *model.AppError を返す。削除したイベントのカテゴリーは、カテゴリーが公開中でも管理画面に出さない。
func findUndeletedEventCategory(ctx context.Context, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, id model.EventCategoryID) (*model.EventCategory, *model.Event, error) {
	category, err := eventCategoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("カテゴリーの取得に失敗: %w", err)
	}
	if category == nil || category.IsDeleted() {
		return nil, nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"event_category_id": id.String()}}
	}

	event, err := findUndeletedEvent(ctx, eventRepo, category.EventID)
	if err != nil {
		return nil, nil, err
	}

	return category, event, nil
}
