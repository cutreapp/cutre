package trade_completion

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/trade"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /trades/{id}/completion - ひとことを入れて「交換できた」を記録する画面を描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
//
// 押せない交換 (マッチ成立でなくなった・すでに押した) では、そのことを伝えて交換のページへ戻す。
// 無い交換と、交換の2人以外には、存在しないページとして404を返す。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。押せる人かを決められないまま描画しない。
		slog.ErrorContext(ctx, "「交換できた」の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tradeID, ok := tradeIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	h.render(w, r, user, tradeID, http.StatusOK, "", nil)
}

// render は、「交換できた」の画面を指定したステータスで描画する。入力の誤りで描き直すときは、送られたひとこと note とエラーを戻す。
//
// 押せない交換では、描画せずに、そのことを伝えて交換のページへ戻す。
// 無い交換と、交換の2人以外には、存在しないページとして404を返す。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, user *model.User, tradeID model.TradeID, status int, note string, formErrors *model.ValidationError) {
	ctx := r.Context()

	output, err := h.getTradeUC.Execute(ctx, usecase.GetTradeInput{ViewerUserID: user.ID, TradeID: tradeID})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "「交換できた」を押す交換の取得に失敗しました", "error", err, "user_id", user.ID, "trade_id", tradeID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	loc, err := user.Location()
	if err != nil {
		slog.ErrorContext(ctx, "ユーザーのタイムゾーンの読み込みに失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	trade := viewmodel.NewTradeDetail(
		ctx, user.ID, output.Trade, output.Partner, output.Items, output.Goods, output.EventCategories, output.Events,
		output.LatestMessage, output.UnreadMessageCount, output.MessageConsentValid, loc, time.Now(),
	)
	if !trade.CanComplete {
		h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_trade_completion_conflict"))
		// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる (gosecのG710は誤検知)。
		//nolint:gosec // G710
		http.Redirect(w, r, templates.TradePath(tradeID.String()), http.StatusSeeOther)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "trade_completion_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavTrade, CurrentPath: templates.TradeCompletionPath(tradeID.String())},
	}
	data := page.CompletionPageData{
		CSRFToken:  middleware.CSRFTokenFromContext(ctx),
		Trade:      trade,
		Note:       note,
		FormErrors: formErrors,
	}
	if err := layouts.Default(layoutData, page.Completion(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "「交換できた」の画面の描画に失敗しました", "error", err)
	}
}
