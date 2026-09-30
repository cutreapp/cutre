package usecase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateItemUsecase はユーザーのリストに、イベントのグッズのアイテムを追加する。
type CreateItemUsecase struct {
	db                *sql.DB
	validator         *validator.ItemCreateValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
	itemRepo          *repository.ItemRepository
}

// NewCreateItemUsecase は CreateItemUsecase を生成する。
func NewCreateItemUsecase(db *sql.DB, validator *validator.ItemCreateValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository, itemRepo *repository.ItemRepository) *CreateItemUsecase {
	return &CreateItemUsecase{db: db, validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo, itemRepo: itemRepo}
}

// CreateItemInput は CreateItemUsecase.Execute の入力。リスト・数量・ひとことはフォームの値をそのまま受け取る。
type CreateItemInput struct {
	UserID   model.UserID
	GoodsID  model.GoodsID
	Kind     string
	Quantity string
	Note     string
}

// CreateItemOutput は CreateItemUsecase.Execute の結果。
type CreateItemOutput struct {
	Item *model.Item
	// EventID と EventCategoryID は、アイテムのグッズのカテゴリーとイベントのID。ハンドラーが戻り先を決めるのに使う。
	EventID         model.EventID
	EventCategoryID model.EventCategoryID
}

// Execute はアイテムをユーザーのリストに入れる。
//
// グッズが無いか、グッズ・カテゴリー・イベントのいずれかが公開中でないときは AppErrCodeResourceNotFound の、
// 同じリストに同じグッズのアイテムが既にあるときは AppErrCodeConflict の *model.AppError を返す。
// 同じアイテムを2つ作らないことはデータベースの一意制約で保証し、二重送信が重なっても1つだけになる。
// フォームの誤りは *model.ValidationError を返す。
func (uc *CreateItemUsecase) Execute(ctx context.Context, input CreateItemInput) (*CreateItemOutput, error) {
	goods, category, event, err := findPublishedGoods(ctx, uc.eventRepo, uc.eventCategoryRepo, uc.goodsRepo, input.GoodsID)
	if err != nil {
		return nil, err
	}

	attrs, err := uc.validator.Validate(ctx, validator.ItemCreateValidatorInput{Kind: input.Kind, Quantity: input.Quantity, Note: input.Note})
	if err != nil {
		return nil, err
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	eventRepo := uc.eventRepo.WithTx(tx)
	categoryRepo := uc.eventCategoryRepo.WithTx(tx)
	goodsRepo := uc.goodsRepo.WithTx(tx)
	itemRepo := uc.itemRepo.WithTx(tx)

	// 親から子の順にロックする。アーカイブ・削除が先に終われば、ロック取得後の読み直しで拒否する。
	if err := eventRepo.LockByID(ctx, event.ID); err != nil {
		return nil, fmt.Errorf("イベントのロックに失敗: %w", err)
	}
	if err := categoryRepo.LockByID(ctx, category.ID); err != nil {
		return nil, fmt.Errorf("カテゴリーのロックに失敗: %w", err)
	}
	if err := goodsRepo.LockByID(ctx, goods.ID); err != nil {
		return nil, fmt.Errorf("グッズのロックに失敗: %w", err)
	}
	if _, _, _, err := findPublishedGoods(ctx, eventRepo, categoryRepo, goodsRepo, goods.ID); err != nil {
		return nil, err
	}

	item, err := itemRepo.Create(ctx, input.UserID, goods.ID, *attrs)
	if err != nil {
		if errors.Is(err, repository.ErrItemAlreadyListed) {
			return nil, &model.AppError{Code: model.AppErrCodeConflict, Internal: err, Metadata: map[string]string{"user_id": input.UserID.String(), "goods_id": goods.ID.String(), "kind": string(attrs.Kind)}}
		}
		return nil, fmt.Errorf("アイテムの追加に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreateItemOutput{Item: item, EventID: event.ID, EventCategoryID: category.ID}, nil
}
