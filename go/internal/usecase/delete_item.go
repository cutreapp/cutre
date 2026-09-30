package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// DeleteItemUsecase はユーザーのリストにあるアイテムをリストから外す。
// 交換の品から辿れるよう、行は消さずに状態を removed にする。
type DeleteItemUsecase struct {
	itemRepo *repository.ItemRepository
}

// NewDeleteItemUsecase は DeleteItemUsecase を生成する。
func NewDeleteItemUsecase(itemRepo *repository.ItemRepository) *DeleteItemUsecase {
	return &DeleteItemUsecase{itemRepo: itemRepo}
}

// DeleteItemInput は DeleteItemUsecase.Execute の入力。
type DeleteItemInput struct {
	UserID model.UserID
	ItemID model.ItemID
	// LockVersion は編集画面を開いたときのアイテムの版。
	LockVersion int32
}

// DeleteItemOutput は DeleteItemUsecase.Execute の結果。
type DeleteItemOutput struct {
	// Item はリストから外す前のアイテム。ハンドラーが戻り先のリスト (Kind) を決めるのに使う。
	Item *model.Item
}

// Execute はアイテムをリストから外す。
//
// アイテムが無いか、リストから外したものか、ほかのユーザーのものであるときは AppErrCodeResourceNotFound の *model.AppError を返す。
// 画面を開いたあとに更新されていたときと、読んだあとに別のタブなどで先に外されたときは、
// どちらも条件に合う行が無いため AppErrCodeConflict を返す。外されていたかは呼び出し側が読み直して見分ける。
func (uc *DeleteItemUsecase) Execute(ctx context.Context, input DeleteItemInput) (*DeleteItemOutput, error) {
	item, err := findListedItem(ctx, uc.itemRepo, input.UserID, input.ItemID)
	if err != nil {
		return nil, err
	}

	removed, err := uc.itemRepo.RemoveListed(ctx, item.ID, input.UserID, input.LockVersion)
	if err != nil {
		return nil, fmt.Errorf("アイテムをリストから外すのに失敗: %w", err)
	}
	if !removed {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"item_id": item.ID.String(), "user_id": input.UserID.String()}}
	}

	return &DeleteItemOutput{Item: item}, nil
}
