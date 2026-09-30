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

// ApproveTradeUsecase は、申し込まれた人が申し込みを承認する。
// 交換をマッチ成立の段階に進め、これまでの流れに承認を記録する。
type ApproveTradeUsecase struct {
	db                 *sql.DB
	messageConsentRepo *repository.MessageConsentRepository
	tradeRepo          *repository.TradeRepository
	tradeEventRepo     *repository.TradeEventRepository
}

// NewApproveTradeUsecase は ApproveTradeUsecase を生成する。
func NewApproveTradeUsecase(
	db *sql.DB,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeEventRepo *repository.TradeEventRepository,
) *ApproveTradeUsecase {
	return &ApproveTradeUsecase{db: db, messageConsentRepo: messageConsentRepo, tradeRepo: tradeRepo, tradeEventRepo: tradeEventRepo}
}

// ApproveTradeInput は ApproveTradeUsecase.Execute の入力。
type ApproveTradeInput struct {
	UserID  model.UserID
	TradeID model.TradeID
}

// Execute は交換 TradeID の申し込みを承認する。
//
// 交換が無いときと、ユーザーが交換の2人のどちらでもないときは AppErrCodeResourceNotFound を、
// 申し込んだ人のときは AppErrCodeForbidden の *model.AppError を返す。
// 返事待ちでなくなっていた (相手が先に取り下げた・別のタブで先に返事をした) ときは、承認せずに AppErrCodeConflict を返す。
// メッセージの取り扱いへの有効な同意が無いときは AppErrCodeMessageConsentRequired を返す。
//
// 返事待ちの交換がある人は同意をやめられないため、同意を確かめてから承認するまでの間に同意が無くなることはない。
func (uc *ApproveTradeUsecase) Execute(ctx context.Context, input ApproveTradeInput) error {
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
	if !tradePolicy.CanReply() {
		return &model.AppError{Code: model.AppErrCodeForbidden, Metadata: metadata}
	}
	// 終わった交換では、同意が無くても同意の案内より先に、返事ができないことを伝える。
	if trade.Status != model.TradeStatusPending {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}

	consent, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
	}
	if consent == nil || !consent.IsValid() {
		return &model.AppError{Code: model.AppErrCodeMessageConsentRequired, Metadata: metadata}
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 返事待ちであることを更新の条件に含め、取り下げやお断りと同時に押されても二重に進めない。
	approved, err := uc.tradeRepo.WithTx(tx).Approve(ctx, trade.ID, input.UserID)
	if err != nil {
		return fmt.Errorf("申し込みの承認に失敗: %w", err)
	}
	if !approved {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}
	if _, err := uc.tradeEventRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, model.TradeEventKindApproved, nil); err != nil {
		return fmt.Errorf("承認の出来事の記録に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	slog.InfoContext(ctx, "交換の申し込みを承認しました", "trade_id", trade.ID, "proposer_user_id", trade.ProposerUserID, "receiver_user_id", trade.ReceiverUserID)

	return nil
}
