package admin_goods

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

// Create POST /admin/categories/{id}/goods - カテゴリーの配下にグッズを作成し、完了のメッセージを付けてカテゴリーの編集の画面へ戻す。
//
// フォームを受け付けなかったときは、送られた値とエラーを付けて作成の画面を描き直す (422)。
// 管理画面を使えないユーザーと、無いカテゴリー・削除したカテゴリー・削除したイベントのカテゴリーには404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないまま作成しない。
		slog.ErrorContext(ctx, "グッズの作成に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	categoryID, ok := eventCategoryIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}
	// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
	// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため、下のリダイレクトでは誤検知。
	editCategoryPath := templates.EditAdminEventCategoryPath(categoryID.String())

	form := viewmodel.MasterForm{Name: r.PostFormValue("name"), Position: r.PostFormValue("position")}
	_, err := h.createGoodsUC.Execute(ctx, usecase.CreateGoodsInput{
		User:            user,
		EventCategoryID: categoryID,
		Name:            form.Name,
		Position:        form.Position,
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			output, getErr := h.getAdminEventCategoryUC.Execute(ctx, usecase.GetAdminEventCategoryInput{User: user, EventCategoryID: categoryID})
			if getErr != nil {
				h.respondError(w, r, getErr, "管理画面のカテゴリーの取得に失敗しました")
				return
			}
			h.renderNew(w, r, user, http.StatusUnprocessableEntity, output, form, ve)
			return
		}
		h.respondError(w, r, err, "グッズの作成に失敗しました")
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_goods_created"))
	//nolint:gosec // G710
	http.Redirect(w, r, editCategoryPath, http.StatusSeeOther)
}
