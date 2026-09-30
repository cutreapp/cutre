package admin_event

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/httpadmin"
	"github.com/cutreapp/cutre/go/internal/httpform"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Delete DELETE /admin/events/{id} - イベントを削除し、完了のメッセージを付けてイベントの一覧へ戻す。
//
// フォームは確認のダイアログからPOSTに _method を載せて届く。
// 削除は管理者だけができ、それ以外のユーザーと、無いイベント・削除したイベントには404を返す。
// リストのアイテムから参照されているときは、アーカイブを案内するエラーを付けて編集の画面を描き直す (422)。
// 画面を開いたあとにほかの操作で更新されていたときは、最新の値で編集の画面を描き直す (409)。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないまま削除しない。
		slog.ErrorContext(ctx, "イベントの削除に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	eventID, ok := eventIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	if err := h.deleteEventUC.Execute(ctx, usecase.DeleteEventInput{User: user, EventID: eventID, LockVersion: httpform.LockVersion(r)}); err != nil {
		status, formErrors := httpadmin.DeleteFailure(err, i18n.T(ctx, "admin_event_conflict_message"))
		if formErrors == nil {
			h.respondError(w, r, err, "イベントの削除に失敗しました")
			return
		}
		output, getErr := h.getAdminEventUC.Execute(ctx, usecase.GetAdminEventInput{User: user, EventID: eventID})
		if getErr != nil {
			h.respondError(w, r, getErr, "管理画面のイベントの取得に失敗しました")
			return
		}
		h.renderEdit(w, r, user, status, output, viewmodel.NewEventForm(output.Event), formErrors)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_event_deleted"))
	http.Redirect(w, r, templates.AdminEventsPath, http.StatusSeeOther)
}
