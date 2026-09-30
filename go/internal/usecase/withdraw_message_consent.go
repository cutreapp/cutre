package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// WithdrawMessageConsentUsecase は、ユーザーのメッセージの取り扱いへの同意をやめる。
// やめると交換の申し込みと承認ができなくなる。記録は消さず、やめた日時を残す。
// 進行中の交換があるときはやめられない。
type WithdrawMessageConsentUsecase struct {
	db                 *sql.DB
	validator          *validator.MessageConsentWithdrawValidator
	messageConsentRepo *repository.MessageConsentRepository
	userRepo           *repository.UserRepository
}

// NewWithdrawMessageConsentUsecase は WithdrawMessageConsentUsecase を生成する。
func NewWithdrawMessageConsentUsecase(
	db *sql.DB,
	validator *validator.MessageConsentWithdrawValidator,
	messageConsentRepo *repository.MessageConsentRepository,
	userRepo *repository.UserRepository,
) *WithdrawMessageConsentUsecase {
	return &WithdrawMessageConsentUsecase{
		db:                 db,
		validator:          validator,
		messageConsentRepo: messageConsentRepo,
		userRepo:           userRepo,
	}
}

// WithdrawMessageConsentInput は WithdrawMessageConsentUsecase.Execute の入力。
type WithdrawMessageConsentInput struct {
	UserID model.UserID
}

// Execute はユーザーのやめていない同意をすべてやめたことにする。
//
// 進行中の交換があるときは *model.ValidationError を返す。
// やめていない同意が無いとき (二重送信や別の画面で先にやめた) は、AppErrCodeConflict の *model.AppError を返す。
//
// ユーザーの行をロックしてから進行中の交換を確かめ、交換の申し込みと直列化する。
// 申し込みが先に終わればその交換を見つけてやめさせず、あとなら申し込みの側が同意の無いことを見つけて止める。
func (uc *WithdrawMessageConsentUsecase) Execute(ctx context.Context, input WithdrawMessageConsentInput) error {
	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	user, err := uc.userRepo.WithTx(tx).LockByID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("ユーザーのロックに失敗: %w", err)
	}
	if user == nil {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"user_id": input.UserID.String()}}
	}
	if err := uc.validator.WithTx(tx).Validate(ctx, input.UserID); err != nil {
		return err
	}

	withdrawn, err := uc.messageConsentRepo.WithTx(tx).WithdrawByUserID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("メッセージの取り扱いへの同意の取りやめに失敗: %w", err)
	}
	if !withdrawn {
		return &model.AppError{Code: model.AppErrCodeConflict}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return nil
}
