package admin_goods_archive

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/httpform"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/admin_goods_archive"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /admin/goods/{id}/archive/new - グッズのアーカイブの理由を入力する画面を描画する。
//
// 既にアーカイブしたグッズは、理由と元に戻すボタンのある編集の画面へ送る。
// 管理画面を使えないユーザーと、無いグッズ・削除したグッズ・削除したカテゴリーやイベントのグッズには404を返す。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。管理画面を開けるかを決められないまま描画しない。
		slog.ErrorContext(ctx, "グッズのアーカイブの画面に現在のユーザーがありません (RequireAuth を通していません)")
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
	if output.Goods.IsArchived() {
		// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
		// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため、ここでは誤検知。
		//nolint:gosec // G710
		http.Redirect(w, r, templates.EditAdminGoodsPath(goodsID.String()), http.StatusSeeOther)
		return
	}

	h.renderNew(w, r, user, http.StatusOK, output, "", nil)
}

// renderNew はグッズのアーカイブの画面を指定したステータスで描画する。
// 理由を受け付けなかったとき (422) にも、送られた理由とエラーと一緒に描き直すのに使う。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, user *model.User, status int, output *usecase.GetAdminGoodsOutput, archiveMessage string, formErrors *model.ValidationError) {
	ctx := r.Context()
	goods := output.Goods

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	if status == http.StatusConflict {
		meta.SetTitle(ctx, "admin_goods_conflict_heading")
	} else {
		meta.SetTitle(ctx, "admin_goods_archive_new_title")
	}
	lockVersion := goods.LockVersion
	if status == http.StatusUnprocessableEntity {
		// 入力を直す間に競合を見落とさないよう、送られた版を持ち回る。
		lockVersion = httpform.LockVersion(r)
	}

	data := page.NewPageData{
		ProfilePath:       templates.ProfilePath(user.Atname),
		CSRFToken:         middleware.CSRFTokenFromContext(ctx),
		EventID:           output.Event.ID.String(),
		EventName:         output.Event.Name,
		EventCategoryID:   output.EventCategory.ID.String(),
		EventCategoryName: output.EventCategory.Name,
		GoodsID:           goods.ID.String(),
		GoodsName:         goods.Name,
		LockVersion:       lockVersion,
		Conflict:          status == http.StatusConflict,
		Archived:          goods.IsArchived(),
		ArchiveMessage:    archiveMessage,
		FormErrors:        formErrors,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.NewAdminGoodsArchivePath(data.GoodsID)},
	}
	if err := layouts.Default(layoutData, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "グッズのアーカイブの画面の描画に失敗しました", "error", err)
	}
}

// renderConflict は最新のグッズを取得し、変更を受け付けなかった理由と現在の状態を409で示す。
func (h *Handler) renderConflict(w http.ResponseWriter, r *http.Request, user *model.User, goodsID model.GoodsID) {
	ctx := r.Context()
	output, err := h.getAdminGoodsUC.Execute(ctx, usecase.GetAdminGoodsInput{User: user, GoodsID: goodsID})
	if err != nil {
		h.respondError(w, r, err, "競合したグッズの取得に失敗しました")
		return
	}
	conflict := model.NewValidationError()
	conflict.AddGlobal(i18n.T(ctx, "admin_goods_conflict_message"))
	h.renderNew(w, r, user, http.StatusConflict, output, "", conflict)
}
