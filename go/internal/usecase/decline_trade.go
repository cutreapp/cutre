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

// DeclineTradeUsecase は、申し込まれた人が、理由を選んで申し込みをお断りする。
// 交換をお断りの段階で終え、これまでの流れに理由とともにお断りを記録し、ひとことがあればメッセージとして相手に届ける。
type DeclineTradeUsecase struct {
	db                 *sql.DB
	validator          *validator.TradeDeclineValidator
	messageConsentRepo *repository.MessageConsentRepository
	tradeRepo          *repository.TradeRepository
	tradeEventRepo     *repository.TradeEventRepository
	tradeMessageRepo   *repository.TradeMessageRepository
}

// NewDeclineTradeUsecase は DeclineTradeUsecase を生成する。
func NewDeclineTradeUsecase(
	db *sql.DB,
	validator *validator.TradeDeclineValidator,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeEventRepo *repository.TradeEventRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
) *DeclineTradeUsecase {
	return &DeclineTradeUsecase{
		db:                 db,
		validator:          validator,
		messageConsentRepo: messageConsentRepo,
		tradeRepo:          tradeRepo,
		tradeEventRepo:     tradeEventRepo,
		tradeMessageRepo:   tradeMessageRepo,
	}
}

// DeclineTradeInput は DeclineTradeUsecase.Execute の入力。理由とひとことはフォームの値をそのまま受け取る。
type DeclineTradeInput struct {
	UserID  model.UserID
	TradeID model.TradeID
	Reason  string
	Note    string
}

// Execute は交換 TradeID の申し込みをお断りする。
//
// 交換が無いときと、ユーザーが交換の2人のどちらでもないときは AppErrCodeResourceNotFound を、
// 申し込んだ人のときは AppErrCodeForbidden の *model.AppError を返す。
// 返事待ちでなくなっていた (相手が先に取り下げた・別のタブで先に返事をした) ときは、お断りせずに AppErrCodeConflict を返す。
// 理由とひとことの誤りは *model.ValidationError で返す。
// ひとことを入れたのに、メッセージの取り扱いへの有効な同意が無いときは AppErrCodeMessageConsentRequired を返す。
//
// 同意が無くても申し込まれうるため、ひとことを入れないお断りには同意を求めない。
// 返事待ちの交換がある人は同意をやめられないため、同意を確かめてからひとことを記録するまでの間に同意が無くなることはない。
func (uc *DeclineTradeUsecase) Execute(ctx context.Context, input DeclineTradeInput) error {
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
	// 終わった交換では、入力の誤りや同意の案内より先に、返事ができないことを伝える。
	if trade.Status != model.TradeStatusPending {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}

	attrs, err := uc.validator.Validate(ctx, validator.TradeDeclineValidatorInput{Reason: input.Reason, Note: input.Note})
	if err != nil {
		return err
	}

	if attrs.Note != "" {
		consent, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.UserID)
		if err != nil {
			return fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
		}
		if consent == nil || !consent.IsValid() {
			return &model.AppError{Code: model.AppErrCodeMessageConsentRequired, Metadata: metadata}
		}
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 返事待ちであることを更新の条件に含め、取り下げや承認と同時に押されても二重に進めない。
	declined, err := uc.tradeRepo.WithTx(tx).Decline(ctx, trade.ID, input.UserID)
	if err != nil {
		return fmt.Errorf("申し込みのお断りに失敗: %w", err)
	}
	if !declined {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}
	reason := string(attrs.Reason)
	if _, err := uc.tradeEventRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, model.TradeEventKindDeclined, &reason); err != nil {
		return fmt.Errorf("お断りの出来事の記録に失敗: %w", err)
	}
	// 交換を終えたのと同じトランザクションで記録するため、段階を条件にしない記録で届ける。
	if attrs.Note != "" {
		if _, err := uc.tradeMessageRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, attrs.Note); err != nil {
			return fmt.Errorf("お断りのひとことの記録に失敗: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	slog.InfoContext(ctx, "交換の申し込みをお断りしました", "trade_id", trade.ID, "proposer_user_id", trade.ProposerUserID, "receiver_user_id", trade.ReceiverUserID, "reason", reason)

	return nil
}
