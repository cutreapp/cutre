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

// DeleteEventUsecase は管理画面でイベントを削除する。
// 行は消さずに削除した状態にし、管理画面にも出さなくする。物理削除はあとからまとめて行う。
type DeleteEventUsecase struct {
	db        *sql.DB
	validator *validator.EventDeleteValidator
	eventRepo *repository.EventRepository
}

// NewDeleteEventUsecase は DeleteEventUsecase を生成する。
func NewDeleteEventUsecase(db *sql.DB, validator *validator.EventDeleteValidator, eventRepo *repository.EventRepository) *DeleteEventUsecase {
	return &DeleteEventUsecase{db: db, validator: validator, eventRepo: eventRepo}
}

// DeleteEventInput は DeleteEventUsecase.Execute の入力。
type DeleteEventInput struct {
	User        *model.User
	EventID     model.EventID
	LockVersion int32
}

// Execute はイベントを削除し、削除したユーザーとイベントをログに残す。
//
// ユーザーが管理者でないときは AppErrCodeForbidden の、イベントが無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
// 配下のグッズを参照するアイテムがあるときは、アーカイブを案内する *model.ValidationError を返す。
// 画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *DeleteEventUsecase) Execute(ctx context.Context, input DeleteEventInput) error {
	if err := authorizeMasterDeletion(input.User); err != nil {
		return err
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	eventRepo := uc.eventRepo.WithTx(tx)
	deleteValidator := uc.validator.WithTx(tx)

	// アイテムの追加と直列化するため、イベントの行をロックしてから参照を確かめる。
	if err := eventRepo.LockByID(ctx, input.EventID); err != nil {
		return fmt.Errorf("イベントのロックに失敗: %w", err)
	}
	if _, err := findUndeletedEvent(ctx, eventRepo, input.EventID); err != nil {
		return err
	}
	if err := deleteValidator.Validate(ctx, input.EventID); err != nil {
		return err
	}

	deleted, err := eventRepo.Delete(ctx, input.EventID, input.LockVersion)
	if err != nil {
		return fmt.Errorf("イベントの削除に失敗: %w", err)
	}
	if !deleted {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_id": input.EventID.String()}}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	slog.InfoContext(ctx, "イベントを削除しました", "user_id", input.User.ID, "event_id", input.EventID)

	return nil
}
