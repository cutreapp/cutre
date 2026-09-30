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

// CompleteTradeUsecase は、交換の2人のどちらかが、マッチ成立の交換で「交換できた」を押す。
// これまでの流れに記録し、ひとことがあればメッセージとして相手に届ける。
// 2人とも押したら交換を「交換できた」の段階で終え、2人のリストの数量を減らす。
type CompleteTradeUsecase struct {
	db                 *sql.DB
	validator          *validator.TradeCompletionValidator
	itemRepo           *repository.ItemRepository
	messageConsentRepo *repository.MessageConsentRepository
	tradeRepo          *repository.TradeRepository
	tradeEventRepo     *repository.TradeEventRepository
	tradeMessageRepo   *repository.TradeMessageRepository
}

// NewCompleteTradeUsecase は CompleteTradeUsecase を生成する。
func NewCompleteTradeUsecase(
	db *sql.DB,
	validator *validator.TradeCompletionValidator,
	itemRepo *repository.ItemRepository,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeEventRepo *repository.TradeEventRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
) *CompleteTradeUsecase {
	return &CompleteTradeUsecase{
		db:                 db,
		validator:          validator,
		itemRepo:           itemRepo,
		messageConsentRepo: messageConsentRepo,
		tradeRepo:          tradeRepo,
		tradeEventRepo:     tradeEventRepo,
		tradeMessageRepo:   tradeMessageRepo,
	}
}

// CompleteTradeInput は CompleteTradeUsecase.Execute の入力。ひとことはフォームの値をそのまま受け取る。
type CompleteTradeInput struct {
	UserID  model.UserID
	TradeID model.TradeID
	Note    string
}

// CompleteTradeOutput は CompleteTradeUsecase.Execute の結果。
type CompleteTradeOutput struct {
	// Completed は、2人とも押しそろって交換を終えたか。falseなら相手が押すのを待っている。
	Completed bool
}

// Execute は交換 TradeID で「交換できた」を押す。
//
// 交換が無いときと、ユーザーが交換の2人のどちらでもないときは AppErrCodeResourceNotFound の *model.AppError を返す。
// マッチ成立でなくなっていた (相手が先にやめた・交換できなかったを記録した) ときと、すでに押していた (別のタブで先に押した) ときは、
// 記録せずに AppErrCodeConflict を返す。
// ひとことの誤りは *model.ValidationError で返す。
// ひとことを入れたのに、メッセージの取り扱いへの有効な同意が無いときは AppErrCodeMessageConsentRequired を返す。
//
// マッチ成立の交換がある人は同意をやめられないため、同意を確かめてからひとことを記録するまでの間に同意が無くなることはない。
func (uc *CompleteTradeUsecase) Execute(ctx context.Context, input CompleteTradeInput) (*CompleteTradeOutput, error) {
	trade, err := uc.tradeRepo.FindByID(ctx, input.TradeID)
	if err != nil {
		return nil, fmt.Errorf("交換の取得に失敗: %w", err)
	}
	metadata := map[string]string{"trade_id": input.TradeID.String(), "user_id": input.UserID.String()}
	if trade == nil || !policy.NewTradePolicy(input.UserID, trade).CanView() {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: metadata}
	}
	// 押せない交換では、入力の誤りや同意の案内より先に、押せないことを伝える。
	if trade.Status != model.TradeStatusMatched || trade.HasCompleted(input.UserID) {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}

	attrs, err := uc.validator.Validate(ctx, validator.TradeCompletionValidatorInput{Note: input.Note})
	if err != nil {
		return nil, err
	}

	if attrs.Note != "" {
		consent, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.UserID)
		if err != nil {
			return nil, fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
		}
		if consent == nil || !consent.IsValid() {
			return nil, &model.AppError{Code: model.AppErrCodeMessageConsentRequired, Metadata: metadata}
		}
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// マッチ成立で、まだ押していないことを更新の条件に含め、2人が同時に押しても、やめる操作と同時に押されても二重に進めない。
	updated, err := uc.tradeRepo.WithTx(tx).Complete(ctx, trade.ID, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("「交換できた」の記録に失敗: %w", err)
	}
	if updated == nil {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}
	if _, err := uc.tradeEventRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, model.TradeEventKindCompleted, nil); err != nil {
		return nil, fmt.Errorf("「交換できた」の出来事の記録に失敗: %w", err)
	}
	// 交換を終えることがあるのと同じトランザクションで記録するため、段階を条件にしない記録で届ける。
	if attrs.Note != "" {
		if _, err := uc.tradeMessageRepo.WithTx(tx).Create(ctx, trade.ID, input.UserID, attrs.Note); err != nil {
			return nil, fmt.Errorf("「交換できた」のひとことの記録に失敗: %w", err)
		}
	}
	completed := updated.Status == model.TradeStatusCompleted
	if completed {
		if err := uc.itemRepo.WithTx(tx).DecrementTraded(ctx, trade.ID); err != nil {
			return nil, fmt.Errorf("交換した品のリストの数量を減らすのに失敗: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	if completed {
		slog.InfoContext(ctx, "交換を「交換できた」で終えました", "trade_id", trade.ID, "user_id", input.UserID, "proposer_user_id", trade.ProposerUserID, "receiver_user_id", trade.ReceiverUserID)
	} else {
		slog.InfoContext(ctx, "「交換できた」を記録しました", "trade_id", trade.ID, "user_id", input.UserID, "proposer_user_id", trade.ProposerUserID, "receiver_user_id", trade.ReceiverUserID)
	}

	return &CompleteTradeOutput{Completed: completed}, nil
}
