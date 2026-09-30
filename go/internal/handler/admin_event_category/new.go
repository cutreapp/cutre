package admin_event_category

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/admin_event_category"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /admin/events/{id}/categories/new - イベントの配下にカテゴリーを作成するフォームを描画する。
// 並び順には、イベントの既存のカテゴリーの最後の値に100を足した値を入れておく。
// 管理画面を使えないユーザーと、無いイベント・削除したイベントには404を返す。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。管理画面を開けるかを決められないまま描画しない。
		slog.ErrorContext(ctx, "管理画面のカテゴリーの作成の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	eventID, ok := eventIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.getAdminEventUC.Execute(ctx, usecase.GetAdminEventInput{User: user, EventID: eventID})
	if err != nil {
		h.respondError(w, r, err, "管理画面のイベントの取得に失敗しました")
		return
	}

	h.renderNew(w, r, user, http.StatusOK, output.Event, viewmodel.NewEventCategoryCreateForm(output.EventCategories), nil)
}

// renderNew はカテゴリーの作成の画面を指定したステータスで描画する。
// 作成のフォームを受け付けなかったとき (422) にも、送られた値とエラーと一緒に描き直すのに使う。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, user *model.User, status int, event *model.Event, form viewmodel.MasterForm, formErrors *model.ValidationError) {
	ctx := r.Context()

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "admin_event_category_new_title")

	data := page.NewPageData{
		ProfilePath: templates.ProfilePath(user.Atname),
		CSRFToken:   middleware.CSRFTokenFromContext(ctx),
		EventID:     event.ID.String(),
		EventName:   event.Name,
		Form:        form,
		FormErrors:  formErrors,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.NewAdminEventCategoryPath(data.EventID)},
	}
	if err := layouts.Default(layoutData, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "管理画面のカテゴリーの作成の画面の描画に失敗しました", "error", err)
	}
}
