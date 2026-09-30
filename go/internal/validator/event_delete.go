package validator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// EventDeleteValidator は管理画面でイベントを削除できるかを検証する。
type EventDeleteValidator struct {
	itemRepo *repository.ItemRepository
}

// NewEventDeleteValidator は EventDeleteValidator を生成する。
func NewEventDeleteValidator(itemRepo *repository.ItemRepository) *EventDeleteValidator {
	return &EventDeleteValidator{itemRepo: itemRepo}
}

// WithTx はtx内でアイテムの参照を確認する新しいValidatorを返す。
func (v *EventDeleteValidator) WithTx(tx *sql.Tx) *EventDeleteValidator {
	return &EventDeleteValidator{itemRepo: v.itemRepo.WithTx(tx)}
}

// Validate は、イベントの配下のカテゴリー・グッズ (状態を問わない) を参照するアイテムが無いかを確かめる。
//
// 外したアイテムも交換の記録から辿るため数える。参照があるときは削除させず、
// 代わりにアーカイブを案内する *model.ValidationError を返す。
func (v *EventDeleteValidator) Validate(ctx context.Context, id model.EventID) error {
	referenced, err := v.itemRepo.ExistsByEventID(ctx, id)
	if err != nil {
		return fmt.Errorf("イベントを参照するアイテムの確認に失敗: %w", err)
	}
	if referenced {
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "admin_event_delete_referenced"))
		return ve
	}

	return nil
}
