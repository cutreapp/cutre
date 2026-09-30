package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// UpdateGoodsUsecase は管理画面でグッズの名前と並び順を更新する。
type UpdateGoodsUsecase struct {
	validator         *validator.GoodsUpdateValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
}

// NewUpdateGoodsUsecase は UpdateGoodsUsecase を生成する。
func NewUpdateGoodsUsecase(validator *validator.GoodsUpdateValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository) *UpdateGoodsUsecase {
	return &UpdateGoodsUsecase{validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo}
}

// UpdateGoodsInput は UpdateGoodsUsecase.Execute の入力。
type UpdateGoodsInput struct {
	User    *model.User
	GoodsID model.GoodsID
	// LockVersion は編集のフォームを開いたときのグッズの版。
	LockVersion int32
	Name        string
	Position    string
}

// UpdateGoodsOutput は UpdateGoodsUsecase.Execute の結果。
type UpdateGoodsOutput struct {
	// EventCategoryID はグッズのカテゴリーのID。ハンドラーが戻り先を決めるのに使う。
	EventCategoryID model.EventCategoryID
}

// Execute はグッズを更新し、更新したユーザーとグッズをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、グッズが無いか、グッズ・カテゴリー・イベントのいずれかを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、フォームの誤りは *model.ValidationError を返す。
// フォームを開いたあとにほかの操作で先に更新されていた (版が一致しない) ときは、上書きせずに
// AppErrCodeConflict の *model.AppError を返す。
func (uc *UpdateGoodsUsecase) Execute(ctx context.Context, input UpdateGoodsInput) (*UpdateGoodsOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	goods, _, _, err := findUndeletedGoods(ctx, uc.eventRepo, uc.eventCategoryRepo, uc.goodsRepo, input.GoodsID)
	if err != nil {
		return nil, err
	}

	attrs, err := uc.validator.Validate(ctx, validator.GoodsUpdateValidatorInput{Name: input.Name, Position: input.Position})
	if err != nil {
		return nil, err
	}

	updated, err := uc.goodsRepo.Update(ctx, input.GoodsID, input.LockVersion, *attrs)
	if err != nil {
		return nil, fmt.Errorf("グッズの更新に失敗: %w", err)
	}
	if !updated {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"goods_id": input.GoodsID.String()}}
	}
	slog.InfoContext(ctx, "グッズを更新しました", "user_id", input.User.ID, "goods_id", input.GoodsID)

	return &UpdateGoodsOutput{EventCategoryID: goods.EventCategoryID}, nil
}
