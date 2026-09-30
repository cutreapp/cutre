package list

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/list"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Index GET /list?kind={kind} - ユーザーの譲れる・ほしいのリストの一方を描画する。
//
// 出すリストはクエリ kind で選び、無いか読めないときは譲れるリストを出す。メインメニューの行き先で、
// 読めない値を404にするとメニューから辿れない壊れたURLが残るため。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のリストかを決められないまま描画しない。
		slog.ErrorContext(ctx, "リストに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	kind, ok := model.ParseItemKind(r.URL.Query().Get("kind"))
	if !ok {
		kind = model.ItemKindGive
	}

	output, err := h.getListUC.Execute(ctx, usecase.GetListInput{UserID: user.ID, Kind: kind})
	if err != nil {
		slog.ErrorContext(ctx, "リストの取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "list_index_title")

	data := page.IndexPageData{
		Kind:       kind,
		Quantities: output.Quantities,
		Sections:   viewmodel.NewListSections(output.Items, output.Goods, output.EventCategories, output.Events),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavList, CurrentPath: templates.ListPath},
	}
	if err := layouts.Default(layoutData, page.Index(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "リストの描画に失敗しました", "error", err)
	}
}
