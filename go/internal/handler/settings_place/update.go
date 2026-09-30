package settings_place

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

// Update PATCH /settings/places - 交換場所を保存し、完了のメッセージを付けてマイページへ戻す。
//
// フォームはPOSTに _method を載せて届く。
// フォームを受け付けなかったとき (画面を開いたあとに選んだ駅がアーカイブ・削除されたときを含む) は、
// 送られた値とエラーを付けて交換場所の画面を描き直す (422)。
// 画面を開いたあとに交換場所が更新されたときは、最新の内容と競合の案内を描き直す (409)。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の交換場所かを決められないまま保存しない。
		slog.ErrorContext(ctx, "交換場所の保存に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// PostFormValue は同じ名前の値を1つしか返さないため、チェックボックスの値は PostForm から読む。
	// PostForm を埋めるため、先に PostFormValue で本文を読ませる。
	form := viewmodel.PlaceForm{PlaceNote: r.PostFormValue("place_note"), LockVersion: httpform.LockVersion(r)}
	form.StationIDs = r.PostForm["station_ids"]

	err := h.updatePlacesUC.Execute(ctx, usecase.UpdatePlacesInput{UserID: user.ID, LockVersion: form.LockVersion, StationIDs: form.StationIDs, PlaceNote: form.PlaceNote})
	if err != nil {
		ve := model.AsValidationError(err)
		ae := model.AsAppError(err)
		if ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		if ve == nil && (ae == nil || ae.Code != model.AppErrCodeConflict) {
			slog.ErrorContext(ctx, "交換場所の保存に失敗しました", "error", err, "user_id", user.ID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		places, getErr := h.getPlacesUC.Execute(ctx, usecase.GetPlacesInput{UserID: user.ID})
		if getErr != nil {
			if ae := model.AsAppError(getErr); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
				h.errorRenderer.NotFound(w, r)
				return
			}
			slog.ErrorContext(ctx, "交換場所の取得に失敗しました", "error", getErr, "user_id", user.ID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if ve != nil {
			h.render(w, r, user, http.StatusUnprocessableEntity, places, form, ve)
			return
		}
		conflict := model.NewValidationError()
		conflict.AddGlobal(i18n.T(ctx, "settings_place_conflict_message"))
		h.render(w, r, user, http.StatusConflict, places, viewmodel.NewPlaceForm(places.Stations, places.User.PlaceNote, places.User.PlaceLockVersion), conflict)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_places_updated"))
	http.Redirect(w, r, templates.ProfilePath(user.Atname), http.StatusSeeOther)
}
