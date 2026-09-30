package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateTradeUsecase は、ユーザーがほかのユーザーに交換を申し込む。
// 交換の品と申し込みの出来事を記録し、ひとことがあればメッセージの1通目として残す。
type CreateTradeUsecase struct {
	db                 *sql.DB
	validator          *validator.TradeCreateValidator
	messageConsentRepo *repository.MessageConsentRepository
	tradeRepo          *repository.TradeRepository
	tradeEventRepo     *repository.TradeEventRepository
	tradeItemRepo      *repository.TradeItemRepository
	tradeMessageRepo   *repository.TradeMessageRepository
	userRepo           *repository.UserRepository
}

// NewCreateTradeUsecase は CreateTradeUsecase を生成する。
func NewCreateTradeUsecase(
	db *sql.DB,
	validator *validator.TradeCreateValidator,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeEventRepo *repository.TradeEventRepository,
	tradeItemRepo *repository.TradeItemRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
	userRepo *repository.UserRepository,
) *CreateTradeUsecase {
	return &CreateTradeUsecase{
		db:                 db,
		validator:          validator,
		messageConsentRepo: messageConsentRepo,
		tradeRepo:          tradeRepo,
		tradeEventRepo:     tradeEventRepo,
		tradeItemRepo:      tradeItemRepo,
		tradeMessageRepo:   tradeMessageRepo,
		userRepo:           userRepo,
	}
}

// CreateTradeInput は CreateTradeUsecase.Execute の入力。選んだアイテムとひとことはフォームの値をそのまま受け取る。
type CreateTradeInput struct {
	ProposerUserID model.UserID
	// Atname は申し込む相手のアットネーム。大文字小文字を区別しない。
	Atname string
	// ReceiveItemIDs はもらうものとして選んだ、相手のアイテムのID。
	ReceiveItemIDs []string
	// GiveItemIDs は渡すものとして選んだ、自分のアイテムのID。
	GiveItemIDs []string
	Note        string
}

// CreateTradeOutput は CreateTradeUsecase.Execute の結果。
type CreateTradeOutput struct {
	Trade *model.Trade
	// Receiver は申し込んだ相手。
	Receiver *model.User
}

// Execute は交換を申し込む。
//
// 相手がいないか退会したときと、自分自身に申し込んだときは AppErrCodeResourceNotFound を返す。
// フォームの誤りと、選べないアイテム (リストから外された・持ち主が違う) を選んだときは *model.ValidationError を返す。
// メッセージの取り扱いへの有効な同意が無いときは AppErrCodeMessageConsentRequired を返す。
func (uc *CreateTradeUsecase) Execute(ctx context.Context, input CreateTradeInput) (*CreateTradeOutput, error) {
	receiver, err := uc.userRepo.FindByAtname(ctx, input.Atname)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if receiver == nil || receiver.ID == input.ProposerUserID {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"atname": input.Atname}}
	}

	trade, err := uc.createTrade(ctx, input, receiver.ID)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "交換を申し込みました", "trade_id", trade.ID, "proposer_user_id", trade.ProposerUserID, "receiver_user_id", trade.ReceiverUserID)

	return &CreateTradeOutput{Trade: trade, Receiver: receiver}, nil
}

// createTrade は、1つのトランザクションで交換・交換の品・申し込みの出来事と、ひとことがあればメッセージを記録する。
//
// 2人のユーザー行をID順にロックして同意をやめる操作と直列化する。
// 選んだアイテムもID順にロックして検証し、リストから外す操作と直列化する。
func (uc *CreateTradeUsecase) createTrade(ctx context.Context, input CreateTradeInput, receiverUserID model.UserID) (*model.Trade, error) {
	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userIDs := []model.UserID{input.ProposerUserID, receiverUserID}
	slices.SortFunc(userIDs, func(a, b model.UserID) int { return strings.Compare(a.String(), b.String()) })
	for _, userID := range userIDs {
		user, err := uc.userRepo.WithTx(tx).LockByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("ユーザーのロックに失敗: %w", err)
		}
		if user == nil {
			return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"user_id": userID.String()}}
		}
	}
	consent, err := uc.messageConsentRepo.WithTx(tx).FindLatestByUserID(ctx, input.ProposerUserID)
	if err != nil {
		return nil, fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
	}
	if consent == nil || !consent.IsValid() {
		return nil, &model.AppError{Code: model.AppErrCodeMessageConsentRequired, Metadata: map[string]string{"user_id": input.ProposerUserID.String()}}
	}

	attrs, err := uc.validator.WithTx(tx).Validate(ctx, validator.TradeCreateValidatorInput{
		ProposerUserID: input.ProposerUserID,
		ReceiverUserID: receiverUserID,
		ReceiveItemIDs: input.ReceiveItemIDs,
		GiveItemIDs:    input.GiveItemIDs,
		Note:           input.Note,
	})
	if err != nil {
		return nil, err
	}

	trade, err := uc.tradeRepo.WithTx(tx).Create(ctx, input.ProposerUserID, receiverUserID)
	if err != nil {
		return nil, fmt.Errorf("交換の作成に失敗: %w", err)
	}
	if err := uc.tradeItemRepo.WithTx(tx).CreateMany(ctx, trade.ID, slices.Concat(attrs.ReceiveItemIDs, attrs.GiveItemIDs)); err != nil {
		return nil, fmt.Errorf("交換の品の作成に失敗: %w", err)
	}
	if _, err := uc.tradeEventRepo.WithTx(tx).Create(ctx, trade.ID, input.ProposerUserID, model.TradeEventKindProposed, nil); err != nil {
		return nil, fmt.Errorf("申し込みの出来事の記録に失敗: %w", err)
	}
	if attrs.Note != "" {
		if _, err := uc.tradeMessageRepo.WithTx(tx).Create(ctx, trade.ID, input.ProposerUserID, attrs.Note); err != nil {
			return nil, fmt.Errorf("申し込みのひとことの記録に失敗: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return trade, nil
}
