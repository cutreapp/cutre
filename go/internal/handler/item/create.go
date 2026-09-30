package item

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Create POST /items - グッズのアイテムをリストに追加し、完了のメッセージを付けてカテゴリーのグッズの一覧へ戻す。
//
// フォームを受け付けなかったときは、送られた値とエラーを付けてリストに追加する画面を描き直す (422)。
// 同じリストに同じグッズが既にあったとき (二重送信や別のタブで先に追加した) は、そのことを伝えて描き直す (409)。
// 無いグッズと、グッズ・カテゴリー・イベントのいずれかを公開していないときは404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のリストに追加するかを決められないまま追加しない。
		slog.ErrorContext(ctx, "リストへの追加に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	goodsID, ok := parseGoodsID(r.PostFormValue("goods_id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	form := viewmodel.ItemForm{Kind: r.PostFormValue("kind"), Quantity: r.PostFormValue("quantity"), Note: r.PostFormValue("note")}
	output, err := h.createItemUC.Execute(ctx, usecase.CreateItemInput{
		UserID:   user.ID,
		GoodsID:  goodsID,
		Kind:     form.Kind,
		Quantity: form.Quantity,
		Note:     form.Note,
	})
	if err != nil {
		if isNotFound(err) {
			h.errorRenderer.NotFound(w, r)
			return
		}

		var status int
		formErrors := model.AsValidationError(err)
		if formErrors != nil {
			status = http.StatusUnprocessableEntity
		} else if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			status = http.StatusConflict
			kind, _ := model.ParseItemKind(form.Kind)
			formErrors = model.NewValidationError()
			formErrors.AddGlobal(i18n.T(ctx, "item_new_already_listed", map[string]any{"List": i18n.T(ctx, viewmodel.ItemKindLabelKey(kind))}))
		} else {
			h.respondInternalError(w, r, err, "リストへの追加に失敗しました")
			return
		}

		getOutput, getErr := h.getGoodsUC.Execute(ctx, usecase.GetGoodsInput{GoodsID: goodsID})
		if getErr != nil {
			h.respondGetGoodsError(w, r, getErr)
			return
		}
		h.renderNew(w, r, user, status, getOutput, form, formErrors)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_item_created", map[string]any{"List": i18n.T(ctx, viewmodel.ItemKindLabelKey(output.Item.Kind))}))
	http.Redirect(w, r, templates.EventCategoryPath(output.EventID.String(), output.EventCategoryID.String()), http.StatusSeeOther)
}
