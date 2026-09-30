package event

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/event"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Index GET /events - 公開中のイベントを開始日の新しい順に描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のリストの数量を出すかを決められないまま描画しない。
		slog.ErrorContext(ctx, "イベントの一覧に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	output, err := h.getEventsUC.Execute(ctx, usecase.GetEventsInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "イベントの一覧の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "event_index_title")

	data := page.IndexPageData{Events: viewmodel.NewEventRows(ctx, output.Events, output.GoodsCounts, output.Quantities)}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavHome, CurrentPath: templates.EventsPath},
	}
	if err := layouts.Default(layoutData, page.Index(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "イベントの一覧の描画に失敗しました", "error", err)
	}
}
