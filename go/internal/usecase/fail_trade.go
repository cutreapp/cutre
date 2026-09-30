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

// FailTradeUsecase は、交換の2人のどちらかが、マッチ成立の交換で理由を選んで「交換できなかった」を記録する。
// 交換を「交換できなかった」の段階で終え、これまでの流れに理由とともに記録し、ひとことがあればメッセージとして相手に届ける。
// リストの数量は変えない。
type FailTradeUsecase struct {
	db                 *sql.DB
	validator          *validator.TradeFailureValidator
	messageConsentRepo *repository.MessageConsentRepository
	tradeRepo          *repository.TradeRepository
	tradeEventRepo     *repository.TradeEventRepository
	tradeMessageRepo   *repository.TradeMessageRepository
}

// NewFailTradeUsecase は FailTradeUsecase を生成する。
func NewFailTradeUsecase(
	db *sql.DB,
	validator *validator.TradeFailureValidator,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeEventRepo *repository.TradeEventRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
) *FailTradeUsecase {
	return &FailTradeUsecase{
		db:                 db,
		validator:          validator,
		messageConsentRepo: messageConsentRepo,
		tradeRepo:          tradeRepo,
		tradeEventRepo:     tradeEventRepo,
		tradeMessageRepo:   tradeMessageRepo,
	}
}

// FailTradeInput は FailTradeUsecase.Execute の入力。理由とひとことはフォームの値をそのまま受け取る。
type FailTradeInput struct {
	UserID  model.UserID
	TradeID model.TradeID
	Reason  string
	Note    string
}

// Execute は交換 TradeID で「交換できなかった」を記録する。
//
// 交換が無いときと、ユーザーが交換の2人のどちらでもないときは AppErrCodeResourceNotFound の *model.AppError を返す。
// マッチ成立でなくなっていた (相手が先にやめた・交換できなかったを記録した・2人そろって交換できた) ときは、記録せずに AppErrCodeConflict を返す。
// どちらかが「交換できた」を押したあとでも、マッチ成立の間は記録できる。
// 理由とひとことの誤りは *model.ValidationError で返す。
// ひとことを入れたのに、メッセージの取り扱いへの有効な同意が無いときは AppErrCodeMessageConsentRequired を返す。
//
// マッチ成立の交換がある人は同意をやめられないため、同意を確かめてからひとことを記録するまでの間に同意が無くなることはない。
func (uc *FailTradeUsecase) Execute(ctx context.Context, input FailTradeInput) error {
	trade, err := uc.tradeRepo.FindByID(ctx, input.TradeID)
	if err != nil {
		return fmt.Errorf("交換の取得に失敗: %w", err)
	}
	metadata := map[string]string{"trade_id": input.TradeID.String(), "user_id": input.UserID.String()}
	if trade == nil || !policy.NewTradePolicy(input.UserID, trade).CanView() {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: metadata}
	}
	// 終わった交換では、入力の誤りや同意の案内より先に、記録できないことを伝える。
	if trade.Status != model.TradeStatusMatched {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}

	attrs, err := uc.validator.Validate(ctx, validator.TradeFailureValidatorInput{Reason: input.Reason, Note: input.Note})
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

	// マッチ成立であることを更新の条件に含め、2人が同時に押しても、「交換できた」で終わるのと同時に押されても二重に進めない。
	failed, err := uc.tradeRepo.WithTx(tx).Fail(ctx, trade.ID, input.UserID)
	if err != nil {
		return fmt.Errorf("「交換できなかった」の記録に失敗: %w", err)
	}
	if !failed {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}
	reason := string(attrs.Reason)
	if _, err := uc.tradeEventRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, model.TradeEventKindFailed, &reason); err != nil {
		return fmt.Errorf("「交換できなかった」の出来事の記録に失敗: %w", err)
	}
	// 交換を終えたのと同じトランザクションで記録するため、段階を条件にしない記録で届ける。
	if attrs.Note != "" {
		if _, err := uc.tradeMessageRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, attrs.Note); err != nil {
			return fmt.Errorf("「交換できなかった」のひとことの記録に失敗: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	slog.InfoContext(ctx, "交換を「交換できなかった」で終えました", "trade_id", trade.ID, "user_id", input.UserID, "proposer_user_id", trade.ProposerUserID, "receiver_user_id", trade.ReceiverUserID, "reason", reason)

	return nil
}
