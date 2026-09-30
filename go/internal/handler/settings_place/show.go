package settings_place

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/settings_place"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET /settings/places - 交換場所の画面を描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の交換場所かを決められないまま描画しない。
		slog.ErrorContext(ctx, "交換場所の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	places, err := h.getPlacesUC.Execute(ctx, usecase.GetPlacesInput{UserID: user.ID})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "交換場所の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.render(w, r, user, http.StatusOK, places, viewmodel.NewPlaceForm(places.Stations, places.User.PlaceNote, places.User.PlaceLockVersion), nil)
}

// render は交換場所の画面を指定したステータスで描画する。
// 駅の選択肢は、公開中の駅と、既に選んでいる駅 places.Stations を合わせたものにし、form の駅にチェックを入れる。
// フォームを受け付けなかったとき (422) は、送られた値を描き直すのに使う。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, user *model.User, status int, places *usecase.GetPlacesOutput, form viewmodel.PlaceForm, formErrors *model.ValidationError) {
	ctx := r.Context()

	published, err := h.getPublishedStationsUC.Execute(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "交換場所の駅の選択肢の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "settings_place_show_title")

	data := page.ShowPageData{
		ProfilePath: templates.ProfilePath(user.Atname),
		CSRFToken:   middleware.CSRFTokenFromContext(ctx),
		Prefectures: viewmodel.NewPlacePrefectures(ctx, published.Stations, places.Stations, form.StationIDs),
		Form:        form,
		FormErrors:  formErrors,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.SettingsPlacesPath},
	}
	if err := layouts.Default(layoutData, page.Show(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "交換場所の画面の描画に失敗しました", "error", err)
	}
}
