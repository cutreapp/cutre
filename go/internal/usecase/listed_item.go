package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// findListedItem は、ユーザー userID が自分のリストにあるアイテムとして扱うアイテムを返す。
//
// アイテムが無いときと、リストから外したものか、ほかのユーザーのものであるときは、そのユーザーには無いものとして
// AppErrCodeResourceNotFound の *model.AppError を返す。ほかのユーザーのアイテムがあることも明かさない。
func findListedItem(ctx context.Context, itemRepo *repository.ItemRepository, userID model.UserID, id model.ItemID) (*model.Item, error) {
	item, err := itemRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("アイテムの取得に失敗: %w", err)
	}
	if item == nil || item.UserID != userID || item.Status != model.ItemStatusListed {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"item_id": id.String(), "user_id": userID.String()}}
	}

	return item, nil
}
