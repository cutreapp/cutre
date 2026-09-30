package item

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/item"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /items/new?goods_id={goods_id}&kind={kind} - グッズをリストに追加するフォームを描画する。
//
// 入れるリストは、カテゴリーのグッズの一覧で押したほう (kind) を選んでおく。kind が無いか読めないときは選ばずに出す。
// 無いグッズと、グッズ・カテゴリー・イベントのいずれかを公開していないときは404を返す。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のリストに追加するかを決められないまま描画しない。
		slog.ErrorContext(ctx, "リストに追加する画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	goodsID, ok := parseGoodsID(r.URL.Query().Get("goods_id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.getGoodsUC.Execute(ctx, usecase.GetGoodsInput{GoodsID: goodsID})
	if err != nil {
		h.respondGetGoodsError(w, r, err)
		return
	}

	kind := r.URL.Query().Get("kind")
	if _, ok := model.ParseItemKind(kind); !ok {
		kind = ""
	}

	h.renderNew(w, r, user, http.StatusOK, output, viewmodel.NewItemForm(kind), nil)
}

// renderNew はリストに追加する画面を指定したステータスで描画する。
// フォームを受け付けなかったとき (422) と、同じリストに既にあったとき (409) にも、送られた値とエラーと一緒に描き直すのに使う。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, user *model.User, status int, output *usecase.GetGoodsOutput, form viewmodel.ItemForm, formErrors *model.ValidationError) {
	ctx := r.Context()

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "item_new_title")

	data := page.NewPageData{
		CSRFToken:         middleware.CSRFTokenFromContext(ctx),
		EventID:           output.Event.ID.String(),
		EventName:         output.Event.Name,
		EventCategoryID:   output.EventCategory.ID.String(),
		EventCategoryName: output.EventCategory.Name,
		GoodsID:           output.Goods.ID.String(),
		GoodsName:         output.Goods.Name,
		Form:              form,
		FormErrors:        formErrors,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavHome, CurrentPath: templates.NewItemPath},
	}
	if err := layouts.Default(layoutData, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "リストに追加する画面の描画に失敗しました", "error", err)
	}
}

// respondInternalError は予期しないエラーをログに残し、500で応える。
func (h *Handler) respondInternalError(w http.ResponseWriter, r *http.Request, err error, logMessage string) {
	slog.ErrorContext(r.Context(), logMessage, "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
