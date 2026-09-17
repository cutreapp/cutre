package welcome

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	welcomepage "github.com/cutreapp/cutre/go/internal/templates/pages/welcome"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET / (日本語版) と GET /en (英語版) - トップページを描画する。
// どちらの言語版も同じ内容のため、ハンドラーを共有して描画する言語だけをcontextのロケールで切り替える。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 言語コードを含まないパスを渡し、canonicalと各言語版のURLはviewmodelに組み立てさせる。
	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, "/")
	meta.SetTitleWithoutSuffix(ctx, "welcome_show_title")
	meta.Description = i18n.T(ctx, "welcome_show_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, welcomepage.Show()).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "トップページの描画に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
