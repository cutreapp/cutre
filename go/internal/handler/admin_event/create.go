package admin_event

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

// Create POST /admin/events - イベントを作成し、完了のメッセージを付けてイベントの一覧へ戻す。
//
// フォームを受け付けなかったときは、送られた値とエラーを付けて作成の画面を描き直す (422)。
// 管理画面を使えないユーザーには404を返す。CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないまま作成しない。
		slog.ErrorContext(ctx, "イベントの作成に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	form := viewmodel.EventForm{
		Name:     r.PostFormValue("name"),
		StartsOn: r.PostFormValue("starts_on"),
		EndsOn:   r.PostFormValue("ends_on"),
	}
	_, err := h.createEventUC.Execute(ctx, usecase.CreateEventInput{
		User:     user,
		Name:     form.Name,
		StartsOn: form.StartsOn,
		EndsOn:   form.EndsOn,
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.renderNew(w, r, user, http.StatusUnprocessableEntity, form, ve)
			return
		}
		h.respondError(w, r, err, "イベントの作成に失敗しました")
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_event_created"))
	http.Redirect(w, r, templates.AdminEventsPath, http.StatusSeeOther)
}
