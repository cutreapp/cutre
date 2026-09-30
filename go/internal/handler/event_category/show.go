package event_category

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/event_category"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET /events/{event_id}/categories/{category_id} - 公開中のカテゴリーの、公開中のグッズを並び順に描画する。
// 無いカテゴリーと、カテゴリーかイベントを公開していないとき、カテゴリーがURLのイベントのものでないときは404を返す。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のリストを出すかを決められないまま描画しない。
		slog.ErrorContext(ctx, "カテゴリーのグッズの一覧に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	eventID, eventErr := uuid.Parse(chi.URLParam(r, "event_id"))
	categoryID, categoryErr := uuid.Parse(chi.URLParam(r, "category_id"))
	if eventErr != nil || categoryErr != nil {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.getEventCategoryUC.Execute(ctx, usecase.GetEventCategoryInput{
		UserID:          user.ID,
		EventID:         model.EventID(eventID),
		EventCategoryID: model.EventCategoryID(categoryID),
	})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "カテゴリーのグッズの一覧の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "event_category_show_title")

	data := page.ShowPageData{
		EventID:           output.Event.ID.String(),
		EventName:         output.Event.Name,
		EventCategoryName: output.EventCategory.Name,
		Goods:             viewmodel.NewGoodsRows(output.Goods, output.Items),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavHome, CurrentPath: templates.EventCategoryPath(data.EventID, output.EventCategory.ID.String())},
	}
	if err := layouts.Default(layoutData, page.Show(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "カテゴリーのグッズの一覧の描画に失敗しました", "error", err)
	}
}
