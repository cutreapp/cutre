package validator

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// tradeMessageBodyMaxLength は交換のメッセージの本文の最大の文字数。
// 申し込みのひとことはメッセージの1通目になるため、同じ上限にする。
const tradeMessageBodyMaxLength = 1000

// TradeCreateValidator は交換の申し込みのフォームを検証する。
type TradeCreateValidator struct {
	itemRepo *repository.ItemRepository
}

// NewTradeCreateValidator は TradeCreateValidator を生成する。
func NewTradeCreateValidator(itemRepo *repository.ItemRepository) *TradeCreateValidator {
	return &TradeCreateValidator{itemRepo: itemRepo}
}

// WithTx は選んだアイテムの確認とロックをtx内で行う新しいValidatorを返す。
func (v *TradeCreateValidator) WithTx(tx *sql.Tx) *TradeCreateValidator {
	return &TradeCreateValidator{itemRepo: v.itemRepo.WithTx(tx)}
}

// TradeCreateValidatorInput は TradeCreateValidator.Validate の入力。選んだアイテムとひとことはフォームの値をそのまま受け取る。
type TradeCreateValidatorInput struct {
	ProposerUserID model.UserID
	ReceiverUserID model.UserID
	// ReceiveItemIDs はもらうものとして選んだ、申し込まれた人のアイテムのID。
	ReceiveItemIDs []string
	// GiveItemIDs は渡すものとして選んだ、申し込んだ人のアイテムのID。
	GiveItemIDs []string
	Note        string
}

// TradeCreateValidateOutput は TradeCreateValidator.Validate の結果。
type TradeCreateValidateOutput struct {
	// ReceiveItemIDs・GiveItemIDs は交換の品にするアイテムのID。同じアイテムを2度送られても1つにまとめる。
	ReceiveItemIDs []model.ItemID
	GiveItemIDs    []model.ItemID
	// Note は前後の空白を除いたひとこと。
	Note string
}

// Validate は交換の申し込みのフォームを検証し、交換の品にするアイテムとひとことを返す。
//
// もらうもの・渡すものはそれぞれ1点以上選ぶ。申し込み内容の確認の画面では選び直す欄を持たないため、選んでいないことはフォーム全体のエラーにする。選べるのは、もらうものは申し込まれた人の、渡すものは申し込んだ人の、
// 譲れるリストにあるアイテム。画面を開いたあとにリストから外されたアイテムや、読めないIDは、選び直しを案内する。
// ひとことは任意で、tradeMessageBodyMaxLength 文字までに限る。入力の誤りは *model.ValidationError で返す。
func (v *TradeCreateValidator) Validate(ctx context.Context, input TradeCreateValidatorInput) (*TradeCreateValidateOutput, error) {
	ve := model.NewValidationError()

	receiveIDs, receiveOK := model.ParseItemIDs(input.ReceiveItemIDs)
	giveIDs, giveOK := model.ParseItemIDs(input.GiveItemIDs)
	if len(receiveIDs) == 0 && receiveOK {
		ve.AddGlobal(i18n.T(ctx, "validation_trade_receive_required"))
	}
	if len(giveIDs) == 0 && giveOK {
		ve.AddGlobal(i18n.T(ctx, "validation_trade_give_required"))
	}

	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > tradeMessageBodyMaxLength {
		ve.AddField("note", i18n.T(ctx, "validation_too_long", map[string]any{"Max": tradeMessageBodyMaxLength}))
	}

	available := receiveOK && giveOK
	if available && len(receiveIDs) > 0 && len(giveIDs) > 0 {
		var err error
		available, err = v.listedGiveItems(ctx, input.ReceiverUserID, receiveIDs, input.ProposerUserID, giveIDs)
		if err != nil {
			return nil, err
		}
	}
	if !available {
		ve.AddGlobal(i18n.T(ctx, "validation_trade_items_unavailable"))
	}

	if ve.HasErrors() {
		return nil, ve
	}

	return &TradeCreateValidateOutput{ReceiveItemIDs: receiveIDs, GiveItemIDs: giveIDs, Note: note}, nil
}

// listedGiveItems は、receiveIDs がどれも receiverUserID の、giveIDs がどれも proposerUserID の、譲れるリストにあるアイテムかを返す。
func (v *TradeCreateValidator) listedGiveItems(ctx context.Context, receiverUserID model.UserID, receiveIDs []model.ItemID, proposerUserID model.UserID, giveIDs []model.ItemID) (bool, error) {
	items, err := v.itemRepo.LockByIDs(ctx, append(append([]model.ItemID{}, receiveIDs...), giveIDs...))
	if err != nil {
		return false, fmt.Errorf("交換に選んだアイテムの取得に失敗: %w", err)
	}
	owners := make(map[model.ItemID]model.UserID, len(items))
	for _, item := range items {
		if item.Status == model.ItemStatusListed && item.Kind == model.ItemKindGive {
			owners[item.ID] = item.UserID
		}
	}

	for _, id := range receiveIDs {
		if owner, ok := owners[id]; !ok || owner != receiverUserID {
			return false, nil
		}
	}
	for _, id := range giveIDs {
		if owner, ok := owners[id]; !ok || owner != proposerUserID {
			return false, nil
		}
	}

	return true, nil
}
