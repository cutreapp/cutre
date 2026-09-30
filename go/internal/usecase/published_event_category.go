package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// findPublishedEventCategory はユーザー向けの画面で扱うカテゴリーと、そのカテゴリーのイベントを返す。
//
// カテゴリーが無いときと、カテゴリーかイベントが公開中でないときは、ユーザー向けの画面に出さないため
// AppErrCodeResourceNotFound の *model.AppError を返す。
func findPublishedEventCategory(ctx context.Context, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, id model.EventCategoryID) (*model.EventCategory, *model.Event, error) {
	category, err := eventCategoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("カテゴリーの取得に失敗: %w", err)
	}
	if category == nil || !category.IsPublished() {
		return nil, nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"event_category_id": id.String()}}
	}

	event, err := findPublishedEvent(ctx, eventRepo, category.EventID)
	if err != nil {
		return nil, nil, err
	}

	return category, event, nil
}
