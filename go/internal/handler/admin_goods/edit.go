package admin_goods

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/admin_goods"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Edit GET /admin/goods/{id}/edit - 管理画面のグッズの編集の画面を描画する。
// 管理画面を使えないユーザーと、無いグッズ・削除したグッズ・削除したカテゴリーやイベントのグッズには404を返す。
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。管理画面を開けるかを決められないまま描画しない。
		slog.ErrorContext(ctx, "管理画面のグッズの編集の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	goodsID, ok := goodsIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.getAdminGoodsUC.Execute(ctx, usecase.GetAdminGoodsInput{User: user, GoodsID: goodsID})
	if err != nil {
		h.respondError(w, r, err, "管理画面のグッズの取得に失敗しました")
		return
	}

	h.renderEdit(w, r, user, http.StatusOK, output, viewmodel.NewGoodsForm(output.Goods), nil)
}

// renderEdit はグッズの編集の画面を指定したステータスで描画する。
//
// 編集のフォームを受け付けなかったとき (422) は送られた値を、ほかの操作で先に更新されていたとき (409) は
// 最新の値を form に入れて、エラーと一緒に描き直すのに使う。見出しとアーカイブ・削除の欄は保存済みのグッズで描く。
func (h *Handler) renderEdit(w http.ResponseWriter, r *http.Request, user *model.User, status int, output *usecase.GetAdminGoodsOutput, form viewmodel.MasterForm, formErrors *model.ValidationError) {
	ctx := r.Context()
	goods := output.Goods

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "admin_goods_edit_title")

	data := page.EditPageData{
		ProfilePath:        templates.ProfilePath(user.Atname),
		CSRFToken:          middleware.CSRFTokenFromContext(ctx),
		EventID:            output.Event.ID.String(),
		EventName:          output.Event.Name,
		EventCategoryID:    output.EventCategory.ID.String(),
		EventCategoryName:  output.EventCategory.Name,
		GoodsID:            goods.ID.String(),
		GoodsName:          goods.Name,
		CurrentLockVersion: goods.LockVersion,
		Form:               form,
		FormErrors:         formErrors,
		Archived:           goods.IsArchived(),
		CanDelete:          output.CanDelete,
	}
	if goods.ArchiveMessage != nil {
		data.ArchiveMessage = *goods.ArchiveMessage
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.EditAdminGoodsPath(data.GoodsID)},
	}
	if err := layouts.Default(layoutData, page.Edit(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "管理画面のグッズの編集の画面の描画に失敗しました", "error", err)
	}
}
