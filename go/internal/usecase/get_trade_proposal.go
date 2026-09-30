package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetTradeProposalUsecase は、ほかのユーザーに交換を申し込む画面 (組み合わせを選ぶ・申し込み内容の確認) に出す、
// 申し込む相手と、その人との間で交換できるアイテムを引く。
type GetTradeProposalUsecase struct {
	eventCategoryRepo  *repository.EventCategoryRepository
	goodsRepo          *repository.GoodsRepository
	itemRepo           *repository.ItemRepository
	messageConsentRepo *repository.MessageConsentRepository
	userRepo           *repository.UserRepository
}

// NewGetTradeProposalUsecase は GetTradeProposalUsecase を生成する。
func NewGetTradeProposalUsecase(
	eventCategoryRepo *repository.EventCategoryRepository,
	goodsRepo *repository.GoodsRepository,
	itemRepo *repository.ItemRepository,
	messageConsentRepo *repository.MessageConsentRepository,
	userRepo *repository.UserRepository,
) *GetTradeProposalUsecase {
	return &GetTradeProposalUsecase{
		eventCategoryRepo:  eventCategoryRepo,
		goodsRepo:          goodsRepo,
		itemRepo:           itemRepo,
		messageConsentRepo: messageConsentRepo,
		userRepo:           userRepo,
	}
}

// GetTradeProposalInput は GetTradeProposalUsecase.Execute の入力。
type GetTradeProposalInput struct {
	// ProposerUserID は申し込もうとしているユーザー。
	ProposerUserID model.UserID
	// Atname は申し込む相手のアットネーム。大文字小文字を区別しない。
	Atname string
}

// GetTradeProposalOutput は GetTradeProposalUsecase.Execute の結果。
type GetTradeProposalOutput struct {
	// Receiver は申し込む相手。
	Receiver *model.User
	// Match は申し込もうとしているユーザーと相手の間で交換できるアイテム。組み合わせの選択肢にする。
	Match model.Match
	// Goods・EventCategories は、交換できるアイテムのグッズと、そのカテゴリー。マスタの状態は問わない。
	Goods           map[model.GoodsID]*model.Goods
	EventCategories map[model.EventCategoryID]*model.EventCategory
	// MessageConsentValid は、申し込もうとしているユーザーにメッセージの取り扱いへの有効な同意があるか。
	// 無いときは申し込めないため、画面は申し込みの代わりにメッセージの利用の画面への案内を出す。
	MessageConsentValid bool
}

// Execute はアットネーム Atname のユーザーに交換を申し込むための情報を返す。
//
// 相手がいないか退会したときと、申し込もうとしているユーザー自身のときは AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetTradeProposalUsecase) Execute(ctx context.Context, input GetTradeProposalInput) (*GetTradeProposalOutput, error) {
	receiver, err := uc.userRepo.FindByAtname(ctx, input.Atname)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if receiver == nil || receiver.ID == input.ProposerUserID {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"atname": input.Atname}}
	}

	consent, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.ProposerUserID)
	if err != nil {
		return nil, fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
	}

	matches, err := findMatches(ctx, uc.itemRepo, uc.goodsRepo, uc.eventCategoryRepo, input.ProposerUserID, []model.UserID{receiver.ID})
	if err != nil {
		return nil, err
	}

	return &GetTradeProposalOutput{
		Receiver:            receiver,
		Match:               matches.Matches[receiver.ID],
		Goods:               matches.Goods,
		EventCategories:     matches.EventCategories,
		MessageConsentValid: consent != nil && consent.IsValid(),
	}, nil
}
