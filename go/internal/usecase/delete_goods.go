package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// DeleteGoodsUsecase は管理画面でグッズを削除する。
// 行は消さずに削除した状態にし、管理画面にも出さなくする。物理削除はあとからまとめて行う。
type DeleteGoodsUsecase struct {
	db                *sql.DB
	validator         *validator.GoodsDeleteValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
}

// NewDeleteGoodsUsecase は DeleteGoodsUsecase を生成する。
func NewDeleteGoodsUsecase(db *sql.DB, validator *validator.GoodsDeleteValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository) *DeleteGoodsUsecase {
	return &DeleteGoodsUsecase{db: db, validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo}
}

// DeleteGoodsInput は DeleteGoodsUsecase.Execute の入力。
type DeleteGoodsInput struct {
	User        *model.User
	GoodsID     model.GoodsID
	LockVersion int32
}

// DeleteGoodsOutput は DeleteGoodsUsecase.Execute の結果。
type DeleteGoodsOutput struct {
	// EventCategoryID は削除したグッズのカテゴリーのID。ハンドラーが戻り先を決めるのに使う。
	EventCategoryID model.EventCategoryID
}

// Execute はグッズを削除し、削除したユーザーとグッズをログに残す。
//
// ユーザーが管理者でないときは AppErrCodeForbidden の、グッズが無いか、グッズ・カテゴリー・イベントのいずれかを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
// グッズを参照するアイテムがあるときは、アーカイブを案内する *model.ValidationError を返す。
// 画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *DeleteGoodsUsecase) Execute(ctx context.Context, input DeleteGoodsInput) (*DeleteGoodsOutput, error) {
	if err := authorizeMasterDeletion(input.User); err != nil {
		return nil, err
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	goodsRepo := uc.goodsRepo.WithTx(tx)
	deleteValidator := uc.validator.WithTx(tx)

	// アイテムの追加と直列化するため、グッズの行をロックしてから参照を確かめる。
	if err := goodsRepo.LockByID(ctx, input.GoodsID); err != nil {
		return nil, fmt.Errorf("グッズのロックに失敗: %w", err)
	}
	goods, _, _, err := findUndeletedGoods(ctx, uc.eventRepo.WithTx(tx), uc.eventCategoryRepo.WithTx(tx), goodsRepo, input.GoodsID)
	if err != nil {
		return nil, err
	}
	if err := deleteValidator.Validate(ctx, input.GoodsID); err != nil {
		return nil, err
	}

	deleted, err := goodsRepo.Delete(ctx, input.GoodsID, input.LockVersion)
	if err != nil {
		return nil, fmt.Errorf("グッズの削除に失敗: %w", err)
	}
	if !deleted {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"goods_id": input.GoodsID.String()}}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	slog.InfoContext(ctx, "グッズを削除しました", "user_id", input.User.ID, "goods_id", input.GoodsID)

	return &DeleteGoodsOutput{EventCategoryID: goods.EventCategoryID}, nil
}
