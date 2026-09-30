package validator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// EventCategoryDeleteValidator は管理画面でカテゴリーを削除できるかを検証する。
type EventCategoryDeleteValidator struct {
	itemRepo *repository.ItemRepository
}

// NewEventCategoryDeleteValidator は EventCategoryDeleteValidator を生成する。
func NewEventCategoryDeleteValidator(itemRepo *repository.ItemRepository) *EventCategoryDeleteValidator {
	return &EventCategoryDeleteValidator{itemRepo: itemRepo}
}

// WithTx はtx内でアイテムの参照を確認する新しいValidatorを返す。
func (v *EventCategoryDeleteValidator) WithTx(tx *sql.Tx) *EventCategoryDeleteValidator {
	return &EventCategoryDeleteValidator{itemRepo: v.itemRepo.WithTx(tx)}
}

// Validate は、カテゴリーの配下のグッズ (状態を問わない) を参照するアイテムが無いかを確かめる。
//
// 外したアイテムも交換の記録から辿るため数える。参照があるときは削除させず、
// 代わりにアーカイブを案内する *model.ValidationError を返す。
func (v *EventCategoryDeleteValidator) Validate(ctx context.Context, id model.EventCategoryID) error {
	referenced, err := v.itemRepo.ExistsByEventCategoryID(ctx, id)
	if err != nil {
		return fmt.Errorf("カテゴリーを参照するアイテムの確認に失敗: %w", err)
	}
	if referenced {
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "admin_event_category_delete_referenced"))
		return ve
	}

	return nil
}
