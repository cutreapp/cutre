package admin_event

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

// Update PATCH /admin/events/{id} - イベントを更新し、完了のメッセージを付けてイベントの一覧へ戻す。
//
// フォームはPOSTに _method を載せて届く。
// フォームを受け付けなかったときは、送られた値とエラーを付けて編集の画面を描き直す (422)。
// フォームを開いたあとにほかの操作で先に更新されていたときは、上書きせずに最新の値で編集の画面を描き直す (409)。
// 管理画面を使えないユーザーと、無いイベント・削除したイベントには404を返す。CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないまま更新しない。
		slog.ErrorContext(ctx, "イベントの更新に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	eventID, ok := eventIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	// 版が読めない送信は、どの版から編集したかが分からないため、古い版からの送信と同じく競合として扱う。
	lockVersion := httpform.LockVersion(r)
	form := viewmodel.EventForm{
		Name:        r.PostFormValue("name"),
		StartsOn:    r.PostFormValue("starts_on"),
		EndsOn:      r.PostFormValue("ends_on"),
		LockVersion: lockVersion,
	}
	err := h.updateEventUC.Execute(ctx, usecase.UpdateEventInput{
		User:        user,
		EventID:     eventID,
		LockVersion: form.LockVersion,
		Name:        form.Name,
		StartsOn:    form.StartsOn,
		EndsOn:      form.EndsOn,
	})
	if err != nil {
		ve := model.AsValidationError(err)
		ae := model.AsAppError(err)
		if ve == nil && (ae == nil || ae.Code != model.AppErrCodeConflict) {
			h.respondError(w, r, err, "イベントの更新に失敗しました")
			return
		}

		// 描き直しに使う保存済みのイベントを引き直す。競合したときは、その最新の値をフォームに入れる。
		output, getErr := h.getAdminEventUC.Execute(ctx, usecase.GetAdminEventInput{User: user, EventID: eventID})
		if getErr != nil {
			h.respondError(w, r, getErr, "管理画面のイベントの取得に失敗しました")
			return
		}
		if ve != nil {
			h.renderEdit(w, r, user, http.StatusUnprocessableEntity, output, form, ve)
			return
		}
		conflict := model.NewValidationError()
		conflict.AddGlobal(i18n.T(ctx, "validation_edit_conflict"))
		h.renderEdit(w, r, user, http.StatusConflict, output, viewmodel.NewEventForm(output.Event), conflict)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_event_updated"))
	http.Redirect(w, r, templates.AdminEventsPath, http.StatusSeeOther)
}
