package admin_goods_archive

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/httpform"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Delete DELETE /admin/goods/{id}/archive - アーカイブしたグッズを公開に戻し、完了のメッセージを付けて編集の画面へ戻す。
//
// フォームはPOSTに _method を載せて届く。もう一度アーカイブすれば元に戻せるため、確認のダイアログは挟まない。
// 画面を開いたあとにほかの操作で更新されていたときは、最新の状態と競合を示す (409)。
// 管理画面を使えないユーザーと、無いグッズ・削除したグッズ・削除したカテゴリーやイベントのグッズには404を返す。CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないまま戻さない。
		slog.ErrorContext(ctx, "グッズを元に戻す操作に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	goodsID, ok := goodsIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}
	// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
	// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため、下のリダイレクトでは誤検知。
	editPath := templates.EditAdminGoodsPath(goodsID.String())

	if err := h.unarchiveGoodsUC.Execute(ctx, usecase.UnarchiveGoodsInput{User: user, GoodsID: goodsID, LockVersion: httpform.LockVersion(r)}); err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			h.renderConflict(w, r, user, goodsID)
			return
		}
		h.respondError(w, r, err, "グッズを元に戻すのに失敗しました")
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_goods_unarchived"))
	//nolint:gosec // G710
	http.Redirect(w, r, editPath, http.StatusSeeOther)
}
