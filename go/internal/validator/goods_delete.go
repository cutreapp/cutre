package validator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GoodsDeleteValidator は管理画面でグッズを削除できるかを検証する。
type GoodsDeleteValidator struct {
	itemRepo *repository.ItemRepository
}

// NewGoodsDeleteValidator は GoodsDeleteValidator を生成する。
func NewGoodsDeleteValidator(itemRepo *repository.ItemRepository) *GoodsDeleteValidator {
	return &GoodsDeleteValidator{itemRepo: itemRepo}
}

// WithTx はtx内でアイテムの参照を確認する新しいValidatorを返す。
func (v *GoodsDeleteValidator) WithTx(tx *sql.Tx) *GoodsDeleteValidator {
	return &GoodsDeleteValidator{itemRepo: v.itemRepo.WithTx(tx)}
}

// Validate は、グッズを参照するアイテムが無いかを確かめる。
//
// 外したアイテムも交換の記録から辿るため数える。参照があるときは削除させず、
// 代わりにアーカイブを案内する *model.ValidationError を返す。
func (v *GoodsDeleteValidator) Validate(ctx context.Context, id model.GoodsID) error {
	referenced, err := v.itemRepo.ExistsByGoodsID(ctx, id)
	if err != nil {
		return fmt.Errorf("グッズを参照するアイテムの確認に失敗: %w", err)
	}
	if referenced {
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "admin_goods_delete_referenced"))
		return ve
	}

	return nil
}
