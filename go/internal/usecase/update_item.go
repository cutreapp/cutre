package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// UpdateItemUsecase はユーザーのリストにあるアイテムの数量とひとことを更新する。
type UpdateItemUsecase struct {
	validator *validator.ItemUpdateValidator
	itemRepo  *repository.ItemRepository
}

// NewUpdateItemUsecase は UpdateItemUsecase を生成する。
func NewUpdateItemUsecase(validator *validator.ItemUpdateValidator, itemRepo *repository.ItemRepository) *UpdateItemUsecase {
	return &UpdateItemUsecase{validator: validator, itemRepo: itemRepo}
}

// UpdateItemInput は UpdateItemUsecase.Execute の入力。数量とひとことはフォームの値をそのまま受け取る。
type UpdateItemInput struct {
	UserID model.UserID
	ItemID model.ItemID
	// LockVersion は編集画面を開いたときのアイテムの版。
	LockVersion int32
	Quantity    string
	Note        string
}

// UpdateItemOutput は UpdateItemUsecase.Execute の結果。
type UpdateItemOutput struct {
	// Item は更新する前のアイテム。ハンドラーが戻り先のリスト (Kind) を決めるのに使う。
	Item *model.Item
}

// Execute はアイテムの数量とひとことを更新する。
//
// アイテムが無いか、リストから外したものか、ほかのユーザーのものであるときは AppErrCodeResourceNotFound の *model.AppError を返す。
// フォームの誤りは *model.ValidationError を返す。
// 画面を開いたあとに更新されていたときと、読んだあとに別のタブなどでリストから外されたときは、
// どちらも条件に合う行が無いため、更新せずに AppErrCodeConflict を返す。外されていたかは呼び出し側が読み直して見分ける。
func (uc *UpdateItemUsecase) Execute(ctx context.Context, input UpdateItemInput) (*UpdateItemOutput, error) {
	item, err := findListedItem(ctx, uc.itemRepo, input.UserID, input.ItemID)
	if err != nil {
		return nil, err
	}

	attrs, err := uc.validator.Validate(ctx, validator.ItemUpdateValidatorInput{Quantity: input.Quantity, Note: input.Note})
	if err != nil {
		return nil, err
	}

	updated, err := uc.itemRepo.UpdateListed(ctx, item.ID, input.UserID, input.LockVersion, *attrs)
	if err != nil {
		return nil, fmt.Errorf("アイテムの更新に失敗: %w", err)
	}
	if !updated {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"item_id": item.ID.String(), "user_id": input.UserID.String()}}
	}

	return &UpdateItemOutput{Item: item}, nil
}
