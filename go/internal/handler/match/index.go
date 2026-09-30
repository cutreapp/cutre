package match

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/match"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Index GET /matches - ユーザーのマッチ候補を描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
//
// 交換から辿る画面としてメインメニューの交換の中に置く。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のマッチ候補かを決められないまま描画しない。
		slog.ErrorContext(ctx, "マッチ候補に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	output, err := h.getMatchesUC.Execute(ctx, usecase.GetMatchesInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "マッチ候補の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "match_index_title")

	data := page.IndexPageData{
		Candidates: viewmodel.NewMatchCandidates(ctx, output.Candidates, output.Matches, output.Stations, output.Goods, output.EventCategories),
		HasPlaces:  output.HasPlaces,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavTrade, CurrentPath: templates.MatchesPath},
	}
	if err := layouts.Default(layoutData, page.Index(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "マッチ候補の描画に失敗しました", "error", err)
	}
}
