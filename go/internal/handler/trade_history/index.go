package trade_history

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/trade"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Index GET /trades/history - ユーザーの終わった交換を、終わった時刻が新しい順に描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の交換かを決められないまま描画しない。
		slog.ErrorContext(ctx, "これまでの交換の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	output, err := h.getTradeHistoryUC.Execute(ctx, usecase.GetTradeHistoryInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "終わった交換の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 終わった日はユーザーのタイムゾーンで示す。
	loc, err := user.Location()
	if err != nil {
		slog.ErrorContext(ctx, "ユーザーのタイムゾーンの読み込みに失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "trade_history_index_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavTrade, CurrentPath: templates.TradeHistoryPath},
	}
	data := page.HistoryPageData{
		Trades: viewmodel.NewTradeHistoryRows(ctx, user.ID, output.Trades, output.Partners, output.Items, loc, time.Now()),
	}
	if err := layouts.Default(layoutData, page.History(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "これまでの交換の画面の描画に失敗しました", "error", err)
	}
}
