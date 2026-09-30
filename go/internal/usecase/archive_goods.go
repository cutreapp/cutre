package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// ArchiveGoodsUsecase は管理画面で公開中のグッズを、理由を残してアーカイブする。
type ArchiveGoodsUsecase struct {
	validator         *validator.GoodsArchiveCreateValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
}

// NewArchiveGoodsUsecase は ArchiveGoodsUsecase を生成する。
func NewArchiveGoodsUsecase(validator *validator.GoodsArchiveCreateValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository) *ArchiveGoodsUsecase {
	return &ArchiveGoodsUsecase{validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo}
}

// ArchiveGoodsInput は ArchiveGoodsUsecase.Execute の入力。
type ArchiveGoodsInput struct {
	User           *model.User
	GoodsID        model.GoodsID
	LockVersion    int32
	ArchiveMessage string
}

// Execute はグッズをアーカイブし、アーカイブしたユーザーとグッズをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、グッズが無いか、グッズ・カテゴリー・イベントのいずれかを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、理由の誤りは *model.ValidationError を返す。
// 既にアーカイブしていたときと、画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *ArchiveGoodsUsecase) Execute(ctx context.Context, input ArchiveGoodsInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	goods, _, _, err := findUndeletedGoods(ctx, uc.eventRepo, uc.eventCategoryRepo, uc.goodsRepo, input.GoodsID)
	if err != nil {
		return err
	}
	if goods.IsArchived() {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"goods_id": input.GoodsID.String()}}
	}

	message, err := uc.validator.Validate(ctx, validator.GoodsArchiveCreateValidatorInput{ArchiveMessage: input.ArchiveMessage})
	if err != nil {
		return err
	}

	archived, err := uc.goodsRepo.Archive(ctx, input.GoodsID, input.LockVersion, message)
	if err != nil {
		return fmt.Errorf("グッズのアーカイブに失敗: %w", err)
	}
	if !archived {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"goods_id": input.GoodsID.String()}}
	}
	slog.InfoContext(ctx, "グッズをアーカイブしました", "user_id", input.User.ID, "goods_id", input.GoodsID)

	return nil
}
