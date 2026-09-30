package trade

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/trade"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// indexCandidateLimit は、交換の画面に先頭から出すマッチ候補の人数。残りはマッチ候補の画面で見る。
const indexCandidateLimit = 3

// Index GET /trades - ユーザーの進行中の交換と、マッチ候補の先頭の何人かと、これまでの交換への行を描画する。メインメニューの交換の行き先。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の交換かを決められないまま描画しない。
		slog.ErrorContext(ctx, "交換の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	trades, err := h.getTradesUC.Execute(ctx, usecase.GetTradesInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "進行中の交換の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	matches, err := h.getMatchesUC.Execute(ctx, usecase.GetMatchesInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "マッチ候補の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	candidates := viewmodel.NewMatchCandidates(ctx, matches.Candidates, matches.Matches, matches.Stations, matches.Goods, matches.EventCategories)

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "trade_index_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavTrade, CurrentPath: templates.TradesPath},
	}
	data := page.IndexPageData{
		Trades:         viewmodel.NewTradeRows(ctx, user.ID, trades.Trades, trades.Partners, trades.Items),
		Candidates:     candidates[:min(len(candidates), indexCandidateLimit)],
		CandidateCount: len(candidates),
		HistoryCounts:  viewmodel.NewTradeHistoryCounts(trades.EndedCounts),
	}
	if err := layouts.Default(layoutData, page.Index(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "交換の画面の描画に失敗しました", "error", err)
	}
}
