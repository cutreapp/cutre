package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// UnarchiveGoodsUsecase は管理画面でアーカイブしたグッズを公開に戻す。
type UnarchiveGoodsUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
}

// NewUnarchiveGoodsUsecase は UnarchiveGoodsUsecase を生成する。
func NewUnarchiveGoodsUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository) *UnarchiveGoodsUsecase {
	return &UnarchiveGoodsUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo}
}

// UnarchiveGoodsInput は UnarchiveGoodsUsecase.Execute の入力。
type UnarchiveGoodsInput struct {
	User        *model.User
	GoodsID     model.GoodsID
	LockVersion int32
}

// Execute はグッズを公開に戻して理由を空にし、戻したユーザーとグッズをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、グッズが無いか、グッズ・カテゴリー・イベントのいずれかを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
// アーカイブしていなかったときと、画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *UnarchiveGoodsUsecase) Execute(ctx context.Context, input UnarchiveGoodsInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	if _, _, _, err := findUndeletedGoods(ctx, uc.eventRepo, uc.eventCategoryRepo, uc.goodsRepo, input.GoodsID); err != nil {
		return err
	}

	unarchived, err := uc.goodsRepo.Unarchive(ctx, input.GoodsID, input.LockVersion)
	if err != nil {
		return fmt.Errorf("グッズを元に戻すのに失敗: %w", err)
	}
	if !unarchived {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"goods_id": input.GoodsID.String()}}
	}
	slog.InfoContext(ctx, "グッズを元に戻しました", "user_id", input.User.ID, "goods_id", input.GoodsID)

	return nil
}
