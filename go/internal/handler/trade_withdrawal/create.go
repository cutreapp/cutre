package trade_withdrawal

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /trades/{id}/withdrawal - 交換の申し込みを取り下げ、完了のメッセージを付けて交換のページへ戻す。
//
// 画面を開いたあとに相手が返事をしていた (別のタブで先に取り下げていた) ときは、取り下げずに、そのことを伝えて交換のページへ戻す。
// 無い交換と、交換の2人以外・申し込まれた人には、存在しないページとして404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰が取り下げるかを決められないまま取り下げない。
		slog.ErrorContext(ctx, "申し込みの取り下げに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	parsed, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.errorRenderer.NotFound(w, r)
		return
	}
	tradeID := model.TradeID(parsed)
	// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
	// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため、下のリダイレクトでは誤検知。
	tradePath := templates.TradePath(tradeID.String())

	if err := h.withdrawTradeUC.Execute(ctx, usecase.WithdrawTradeInput{UserID: user.ID, TradeID: tradeID}); err != nil {
		if ae := model.AsAppError(err); ae != nil {
			switch ae.Code {
			case model.AppErrCodeResourceNotFound, model.AppErrCodeForbidden:
				h.errorRenderer.NotFound(w, r)
				return
			case model.AppErrCodeConflict:
				h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_trade_withdrawal_conflict"))
				//nolint:gosec // G710
				http.Redirect(w, r, tradePath, http.StatusSeeOther)
				return
			}
		}
		slog.ErrorContext(ctx, "申し込みの取り下げに失敗しました", "error", err, "user_id", user.ID, "trade_id", tradeID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_trade_withdrawn"))
	//nolint:gosec // G710
	http.Redirect(w, r, tradePath, http.StatusSeeOther)
}
