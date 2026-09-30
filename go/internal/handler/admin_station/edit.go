package admin_station

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/admin_station"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Edit GET /admin/stations/{id}/edit - 管理画面の駅の編集の画面を描画する。
// 管理画面を使えないユーザーと、無い駅・削除した駅には404を返す。
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。管理画面を開けるかを決められないまま描画しない。
		slog.ErrorContext(ctx, "管理画面の駅の編集の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	stationID, ok := stationIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.getAdminStationUC.Execute(ctx, usecase.GetAdminStationInput{User: user, StationID: stationID})
	if err != nil {
		h.respondError(w, r, err, "管理画面の駅の取得に失敗しました")
		return
	}

	h.renderEdit(w, r, user, http.StatusOK, output, viewmodel.NewStationForm(output.Station), nil)
}

// renderEdit は駅の編集の画面を指定したステータスで描画する。
//
// 編集のフォームを受け付けなかったとき (422) は送られた値を、ほかの操作で先に更新されていたとき (409) は
// 最新の値を form に入れて、エラーと一緒に描き直すのに使う。見出しとアーカイブ・削除の欄は保存済みの駅で描く。
func (h *Handler) renderEdit(w http.ResponseWriter, r *http.Request, user *model.User, status int, output *usecase.GetAdminStationOutput, form viewmodel.StationForm, formErrors *model.ValidationError) {
	ctx := r.Context()
	station := output.Station

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "admin_station_edit_title")

	data := page.EditPageData{
		ProfilePath:        templates.ProfilePath(user.Atname),
		CSRFToken:          middleware.CSRFTokenFromContext(ctx),
		StationID:          station.ID.String(),
		StationName:        station.Name,
		CurrentLockVersion: station.LockVersion,
		Prefectures:        viewmodel.NewPrefectureOptions(ctx),
		Form:               form,
		FormErrors:         formErrors,
		Archived:           station.IsArchived(),
		CanDelete:          output.CanDelete,
	}
	if station.ArchiveMessage != nil {
		data.ArchiveMessage = *station.ArchiveMessage
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.EditAdminStationPath(data.StationID)},
	}
	if err := layouts.Default(layoutData, page.Edit(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "管理画面の駅の編集の画面の描画に失敗しました", "error", err)
	}
}
