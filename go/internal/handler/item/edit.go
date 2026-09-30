package item

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/item"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Edit GET /items/{id}/edit - リストにあるアイテムの数量とひとことを編集するフォームを描画する。
//
// 無いアイテムと、リストから外したアイテム・ほかのユーザーのアイテムには404を返す。
// アイテムのグッズをアーカイブしていても、リストにある限り編集できる。
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のアイテムかを決められないまま描画しない。
		slog.ErrorContext(ctx, "アイテムの編集の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	itemID, ok := itemIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.getItemUC.Execute(ctx, usecase.GetItemInput{UserID: user.ID, ItemID: itemID})
	if err != nil {
		h.respondItemError(w, r, err, "アイテムの取得に失敗しました")
		return
	}

	h.renderEdit(w, r, user, http.StatusOK, output, viewmodel.NewItemEditForm(output.Item), nil)
}

// renderEdit はアイテムの編集の画面を指定したステータスで描画する。
// フォームを受け付けなかったとき (422) は送られた値を、競合したとき (409) は最新の値を描き直すのに使う。
func (h *Handler) renderEdit(w http.ResponseWriter, r *http.Request, user *model.User, status int, output *usecase.GetItemOutput, form viewmodel.ItemForm, formErrors *model.ValidationError) {
	ctx := r.Context()

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "item_edit_title")

	data := page.EditPageData{
		CSRFToken:         middleware.CSRFTokenFromContext(ctx),
		ItemID:            output.Item.ID.String(),
		Kind:              output.Item.Kind,
		EventName:         output.Event.Name,
		EventCategoryName: output.EventCategory.Name,
		GoodsName:         output.Goods.Name,
		Form:              form,
		FormErrors:        formErrors,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavList, CurrentPath: templates.EditItemPath(data.ItemID)},
	}
	if err := layouts.Default(layoutData, page.Edit(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "アイテムの編集の画面の描画に失敗しました", "error", err)
	}
}

// respondItemError は、リストにあるアイテムを扱うUseCaseのエラーに応える。
// アイテムが無いか、リストから外したものか、ほかのユーザーのものであるときは404を、それ以外は logMessage をログに残して500を返す。
func (h *Handler) respondItemError(w http.ResponseWriter, r *http.Request, err error, logMessage string) {
	if isNotFound(err) {
		h.errorRenderer.NotFound(w, r)
		return
	}

	h.respondInternalError(w, r, err, logMessage)
}
