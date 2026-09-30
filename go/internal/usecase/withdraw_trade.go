package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// WithdrawTradeUsecase は、申し込んだ人が、申し込まれた人の返事の前に申し込みを取り下げる。
// 交換を取り下げの段階で終え、これまでの流れに取り下げを記録する。
type WithdrawTradeUsecase struct {
	db             *sql.DB
	tradeRepo      *repository.TradeRepository
	tradeEventRepo *repository.TradeEventRepository
}

// NewWithdrawTradeUsecase は WithdrawTradeUsecase を生成する。
func NewWithdrawTradeUsecase(db *sql.DB, tradeRepo *repository.TradeRepository, tradeEventRepo *repository.TradeEventRepository) *WithdrawTradeUsecase {
	return &WithdrawTradeUsecase{db: db, tradeRepo: tradeRepo, tradeEventRepo: tradeEventRepo}
}

// WithdrawTradeInput は WithdrawTradeUsecase.Execute の入力。
type WithdrawTradeInput struct {
	UserID  model.UserID
	TradeID model.TradeID
}

// Execute は交換 TradeID の申し込みを取り下げる。
//
// 交換が無いときと、ユーザーが交換の2人のどちらでもないときは AppErrCodeResourceNotFound を、
// 申し込まれた人のときは AppErrCodeForbidden の *model.AppError を返す。
// 返事待ちでなくなっていた (相手が先に返事をした・別のタブで先に取り下げた) ときは、取り下げずに AppErrCodeConflict を返す。
func (uc *WithdrawTradeUsecase) Execute(ctx context.Context, input WithdrawTradeInput) error {
	trade, err := uc.tradeRepo.FindByID(ctx, input.TradeID)
	if err != nil {
		return fmt.Errorf("交換の取得に失敗: %w", err)
	}
	metadata := map[string]string{"trade_id": input.TradeID.String(), "user_id": input.UserID.String()}
	if trade == nil {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: metadata}
	}
	tradePolicy := policy.NewTradePolicy(input.UserID, trade)
	if !tradePolicy.CanView() {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: metadata}
	}
	if !tradePolicy.CanWithdraw() {
		return &model.AppError{Code: model.AppErrCodeForbidden, Metadata: metadata}
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 返事待ちであることを更新の条件に含め、承認やお断りと同時に押されても二重に進めない。
	withdrawn, err := uc.tradeRepo.WithTx(tx).Withdraw(ctx, trade.ID, input.UserID)
	if err != nil {
		return fmt.Errorf("申し込みの取り下げに失敗: %w", err)
	}
	if !withdrawn {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}
	if _, err := uc.tradeEventRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, model.TradeEventKindWithdrawn, nil); err != nil {
		return fmt.Errorf("取り下げの出来事の記録に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	slog.InfoContext(ctx, "交換の申し込みを取り下げました", "trade_id", trade.ID, "proposer_user_id", trade.ProposerUserID, "receiver_user_id", trade.ReceiverUserID)

	return nil
}
