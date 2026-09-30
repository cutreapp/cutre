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

// DeleteEventCategoryUsecase は管理画面でカテゴリーを削除する。
// 行は消さずに削除した状態にし、管理画面にも出さなくする。物理削除はあとからまとめて行う。
type DeleteEventCategoryUsecase struct {
	db                *sql.DB
	validator         *validator.EventCategoryDeleteValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
}

// NewDeleteEventCategoryUsecase は DeleteEventCategoryUsecase を生成する。
func NewDeleteEventCategoryUsecase(db *sql.DB, validator *validator.EventCategoryDeleteValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository) *DeleteEventCategoryUsecase {
	return &DeleteEventCategoryUsecase{db: db, validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo}
}

// DeleteEventCategoryInput は DeleteEventCategoryUsecase.Execute の入力。
type DeleteEventCategoryInput struct {
	User            *model.User
	EventCategoryID model.EventCategoryID
	LockVersion     int32
}

// DeleteEventCategoryOutput は DeleteEventCategoryUsecase.Execute の結果。
type DeleteEventCategoryOutput struct {
	// EventID は削除したカテゴリーのイベントのID。ハンドラーが戻り先を決めるのに使う。
	EventID model.EventID
}

// Execute はカテゴリーを削除し、削除したユーザーとカテゴリーをログに残す。
//
// ユーザーが管理者でないときは AppErrCodeForbidden の、カテゴリーが無いか、カテゴリーかイベントを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
// 配下のグッズを参照するアイテムがあるときは、アーカイブを案内する *model.ValidationError を返す。
// 画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *DeleteEventCategoryUsecase) Execute(ctx context.Context, input DeleteEventCategoryInput) (*DeleteEventCategoryOutput, error) {
	if err := authorizeMasterDeletion(input.User); err != nil {
		return nil, err
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	categoryRepo := uc.eventCategoryRepo.WithTx(tx)
	deleteValidator := uc.validator.WithTx(tx)

	// アイテムの追加と直列化するため、カテゴリーの行をロックしてから参照を確かめる。
	if err := categoryRepo.LockByID(ctx, input.EventCategoryID); err != nil {
		return nil, fmt.Errorf("カテゴリーのロックに失敗: %w", err)
	}
	category, _, err := findUndeletedEventCategory(ctx, uc.eventRepo.WithTx(tx), categoryRepo, input.EventCategoryID)
	if err != nil {
		return nil, err
	}
	if err := deleteValidator.Validate(ctx, input.EventCategoryID); err != nil {
		return nil, err
	}

	deleted, err := categoryRepo.Delete(ctx, input.EventCategoryID, input.LockVersion)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーの削除に失敗: %w", err)
	}
	if !deleted {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_category_id": input.EventCategoryID.String()}}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	slog.InfoContext(ctx, "カテゴリーを削除しました", "user_id", input.User.ID, "event_category_id", input.EventCategoryID)

	return &DeleteEventCategoryOutput{EventID: category.EventID}, nil
}
