package item

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/httpform"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Delete DELETE /items/{id} - アイテムをリストから外し、完了のメッセージを付けてアイテムのあったリストへ戻す。
//
// フォームはPOSTに _method を載せて届く。アイテムの行は消さず、状態を removed にする。
// 画面を開いたあとに変更されたときは、最新の値と競合の案内を描き直す (409)。
// 競合のあとに読み直してアイテムが無ければ (別のタブなどで先に外されていれば)、404を返す。
// 無いアイテムと、リストから外したアイテム・ほかのユーザーのアイテムには404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のアイテムかを決められないまま外さない。
		slog.ErrorContext(ctx, "アイテムをリストから外す操作に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	itemID, ok := itemIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.deleteItemUC.Execute(ctx, usecase.DeleteItemInput{UserID: user.ID, ItemID: itemID, LockVersion: httpform.LockVersion(r)})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			getOutput, getErr := h.getItemUC.Execute(ctx, usecase.GetItemInput{UserID: user.ID, ItemID: itemID})
			if getErr != nil {
				h.respondItemError(w, r, getErr, "アイテムの取得に失敗しました")
				return
			}
			conflict := model.NewValidationError()
			conflict.AddGlobal(i18n.T(ctx, "item_edit_conflict_message"))
			h.renderEdit(w, r, user, http.StatusConflict, getOutput, viewmodel.NewItemEditForm(getOutput.Item), conflict)
			return
		}
		h.respondItemError(w, r, err, "アイテムをリストから外すのに失敗しました")
		return
	}

	kind := output.Item.Kind
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_item_deleted", map[string]any{"List": i18n.T(ctx, viewmodel.ItemKindLabelKey(kind))}))
	http.Redirect(w, r, templates.ListKindPath(string(kind)), http.StatusSeeOther)
}
