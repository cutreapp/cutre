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

// Update PATCH /items/{id} - アイテムの数量とひとことを更新し、完了のメッセージを付けてアイテムのリストへ戻す。
//
// フォームはPOSTに _method を載せて届く。
// フォームを受け付けなかったときは、送られた値とエラーを付けて編集の画面を描き直す (422)。
// 画面を開いたあとに変更されたときは、最新の値と競合の案内を描き直す (409)。
// 競合のあとに読み直してアイテムが無ければ (別のタブなどで外されていれば)、404を返す。
// 無いアイテムと、リストから外したアイテム・ほかのユーザーのアイテムには404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のアイテムかを決められないまま更新しない。
		slog.ErrorContext(ctx, "アイテムの更新に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	itemID, ok := itemIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	form := viewmodel.ItemForm{Quantity: r.PostFormValue("quantity"), Note: r.PostFormValue("note"), LockVersion: httpform.LockVersion(r)}
	output, err := h.updateItemUC.Execute(ctx, usecase.UpdateItemInput{
		UserID:      user.ID,
		ItemID:      itemID,
		LockVersion: form.LockVersion,
		Quantity:    form.Quantity,
		Note:        form.Note,
	})
	if err != nil {
		ve := model.AsValidationError(err)
		ae := model.AsAppError(err)
		if ve == nil && (ae == nil || ae.Code != model.AppErrCodeConflict) {
			h.respondItemError(w, r, err, "アイテムの更新に失敗しました")
			return
		}

		getOutput, getErr := h.getItemUC.Execute(ctx, usecase.GetItemInput{UserID: user.ID, ItemID: itemID})
		if getErr != nil {
			h.respondItemError(w, r, getErr, "アイテムの取得に失敗しました")
			return
		}
		if ve != nil {
			form.Kind = string(getOutput.Item.Kind)
			h.renderEdit(w, r, user, http.StatusUnprocessableEntity, getOutput, form, ve)
			return
		}
		conflict := model.NewValidationError()
		conflict.AddGlobal(i18n.T(ctx, "item_edit_conflict_message"))
		h.renderEdit(w, r, user, http.StatusConflict, getOutput, viewmodel.NewItemEditForm(getOutput.Item), conflict)
		return
	}

	kind := output.Item.Kind
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_item_updated", map[string]any{"List": i18n.T(ctx, viewmodel.ItemKindLabelKey(kind))}))
	http.Redirect(w, r, templates.ListKindPath(string(kind)), http.StatusSeeOther)
}
