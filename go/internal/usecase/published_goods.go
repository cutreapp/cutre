package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// findPublishedGoods はユーザー向けの画面で扱うグッズと、そのグッズのカテゴリーとイベントを返す。
//
// グッズが無いときと、グッズ・カテゴリー・イベントのいずれかが公開中でないときは、ユーザー向けの画面に出さないため
// AppErrCodeResourceNotFound の *model.AppError を返す。
func findPublishedGoods(ctx context.Context, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository, id model.GoodsID) (*model.Goods, *model.EventCategory, *model.Event, error) {
	goods, err := goodsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("グッズの取得に失敗: %w", err)
	}
	if goods == nil || !goods.IsPublished() {
		return nil, nil, nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"goods_id": id.String()}}
	}

	category, event, err := findPublishedEventCategory(ctx, eventRepo, eventCategoryRepo, goods.EventCategoryID)
	if err != nil {
		return nil, nil, nil, err
	}

	return goods, category, event, nil
}
