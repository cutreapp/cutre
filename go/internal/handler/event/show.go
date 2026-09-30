package event

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
	page "github.com/cutreapp/cutre/go/internal/templates/pages/event"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET /events/{event_id} - 公開中のイベントの、公開中のカテゴリーを並び順に描画する。
// 無いイベントと、アーカイブ・削除したイベントには404を返す。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のリストの数量を出すかを決められないまま描画しない。
		slog.ErrorContext(ctx, "イベントのカテゴリーの一覧に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	eventID, err := uuid.Parse(chi.URLParam(r, "event_id"))
	if err != nil {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.getEventUC.Execute(ctx, usecase.GetEventInput{UserID: user.ID, EventID: model.EventID(eventID)})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "イベントのカテゴリーの一覧の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "event_show_title")

	var goodsCount int64
	for _, count := range output.GoodsCounts {
		goodsCount += count
	}
	data := page.ShowPageData{
		EventID:         output.Event.ID.String(),
		EventName:       output.Event.Name,
		Period:          viewmodel.EventPeriod(ctx, output.Event),
		GoodsCount:      goodsCount,
		EventCategories: viewmodel.NewEventCategoryRows(output.EventCategories, output.GoodsCounts, output.Quantities),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavHome, CurrentPath: templates.EventPath(data.EventID)},
	}
	if err := layouts.Default(layoutData, page.Show(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "イベントのカテゴリーの一覧の描画に失敗しました", "error", err)
	}
}
