package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CancelTradeUsecase は、交換の2人のどちらかが、どちらも「交換できた」を押していないマッチ成立の交換を、
// 理由とひとことを添えてやめる。
// 交換を「やめた」の段階で終え、これまでの流れに理由とともに記録し、ひとことをメッセージとして相手に届ける。
type CancelTradeUsecase struct {
	db                 *sql.DB
	validator          *validator.TradeCancellationValidator
	messageConsentRepo *repository.MessageConsentRepository
	tradeRepo          *repository.TradeRepository
	tradeEventRepo     *repository.TradeEventRepository
	tradeMessageRepo   *repository.TradeMessageRepository
}

// NewCancelTradeUsecase は CancelTradeUsecase を生成する。
func NewCancelTradeUsecase(
	db *sql.DB,
	validator *validator.TradeCancellationValidator,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeEventRepo *repository.TradeEventRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
) *CancelTradeUsecase {
	return &CancelTradeUsecase{
		db:                 db,
		validator:          validator,
		messageConsentRepo: messageConsentRepo,
		tradeRepo:          tradeRepo,
		tradeEventRepo:     tradeEventRepo,
		tradeMessageRepo:   tradeMessageRepo,
	}
}

// CancelTradeInput は CancelTradeUsecase.Execute の入力。理由とひとことはフォームの値をそのまま受け取る。
type CancelTradeInput struct {
	UserID  model.UserID
	TradeID model.TradeID
	Reason  string
	Note    string
}

// Execute は交換 TradeID をやめる。
//
// 交換が無いときと、ユーザーが交換の2人のどちらでもないときは AppErrCodeResourceNotFound の *model.AppError を返す。
// マッチ成立でなくなっていた (相手が先にやめた・交換できなかったを記録した) ときと、どちらかが「交換できた」を押していたときは、
// やめずに AppErrCodeConflict を返す。
// 理由とひとことの誤りは *model.ValidationError で返す。
// ひとことは必須のため、メッセージの取り扱いへの有効な同意が無いときは AppErrCodeMessageConsentRequired を返す。
//
// マッチ成立の交換がある人は同意をやめられないため、同意を確かめてからひとことを記録するまでの間に同意が無くなることはない。
func (uc *CancelTradeUsecase) Execute(ctx context.Context, input CancelTradeInput) error {
	trade, err := uc.tradeRepo.FindByID(ctx, input.TradeID)
	if err != nil {
		return fmt.Errorf("交換の取得に失敗: %w", err)
	}
	metadata := map[string]string{"trade_id": input.TradeID.String(), "user_id": input.UserID.String()}
	if trade == nil || !policy.NewTradePolicy(input.UserID, trade).CanView() {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: metadata}
	}
	// やめられない交換では、入力の誤りや同意の案内より先に、やめられないことを伝える。
	if trade.Status != model.TradeStatusMatched || trade.HasAnyCompleted() {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}

	attrs, err := uc.validator.Validate(ctx, validator.TradeCancellationValidatorInput{Reason: input.Reason, Note: input.Note})
	if err != nil {
		return err
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

	// マッチ成立で、どちらも「交換できた」を押していないことを更新の条件に含め、「交換できた」や「交換できなかった」と同時に押されても二重に進めない。
	cancelled, err := uc.tradeRepo.WithTx(tx).Cancel(ctx, trade.ID, input.UserID)
	if err != nil {
		return fmt.Errorf("交換をやめるのに失敗: %w", err)
	}
	if !cancelled {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}
	reason := string(attrs.Reason)
	if _, err := uc.tradeEventRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, model.TradeEventKindCancelled, &reason); err != nil {
		return fmt.Errorf("交換をやめた出来事の記録に失敗: %w", err)
	}
	// 交換を終えたのと同じトランザクションで記録するため、段階を条件にしない記録で届ける。
	if _, err := uc.tradeMessageRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, attrs.Note); err != nil {
		return fmt.Errorf("交換をやめたひとことの記録に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	slog.InfoContext(ctx, "交換をやめました", "trade_id", trade.ID, "user_id", input.UserID, "proposer_user_id", trade.ProposerUserID, "receiver_user_id", trade.ReceiverUserID, "reason", reason)

	return nil
}
