package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateGoodsUsecase は管理画面でカテゴリーの配下にグッズを作成する。
type CreateGoodsUsecase struct {
	validator         *validator.GoodsCreateValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
}

// NewCreateGoodsUsecase は CreateGoodsUsecase を生成する。
func NewCreateGoodsUsecase(validator *validator.GoodsCreateValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository) *CreateGoodsUsecase {
	return &CreateGoodsUsecase{validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo}
}

// CreateGoodsInput は CreateGoodsUsecase.Execute の入力。並び順はフォームの値をそのまま受け取る。
type CreateGoodsInput struct {
	User            *model.User
	EventCategoryID model.EventCategoryID
	Name            string
	Position        string
}

// CreateGoodsOutput は CreateGoodsUsecase.Execute の結果。
type CreateGoodsOutput struct {
	Goods *model.Goods
}

// Execute は公開中のグッズを作成し、作成したユーザーとグッズをログに残す。
// アーカイブしたカテゴリーやイベントにも、公開に戻すときに備えてグッズを作れる。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、カテゴリーが無いか、カテゴリーかイベントを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、フォームの誤りは *model.ValidationError を返す。
func (uc *CreateGoodsUsecase) Execute(ctx context.Context, input CreateGoodsInput) (*CreateGoodsOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	if _, _, err := findUndeletedEventCategory(ctx, uc.eventRepo, uc.eventCategoryRepo, input.EventCategoryID); err != nil {
		return nil, err
	}

	attrs, err := uc.validator.Validate(ctx, validator.GoodsCreateValidatorInput{Name: input.Name, Position: input.Position})
	if err != nil {
		return nil, err
	}

	goods, err := uc.goodsRepo.Create(ctx, input.EventCategoryID, *attrs)
	if err != nil {
		return nil, fmt.Errorf("グッズの作成に失敗: %w", err)
	}
	slog.InfoContext(ctx, "グッズを作成しました", "user_id", input.User.ID, "event_category_id", input.EventCategoryID, "goods_id", goods.ID)

	return &CreateGoodsOutput{Goods: goods}, nil
}
