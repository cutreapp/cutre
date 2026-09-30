package trade_cancellation

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /trades/{id}/cancellation - 理由とひとことを添えて交換をやめ、完了のメッセージを付けて交換のページへ戻す。
//
// フォームを受け付けなかったときは、送られた値とエラーを付けて交換をやめる画面を描き直す (422)。
// 画面を開いたあとに交換が終わっていたか、どちらかが「交換できた」を押していたときは、やめずに、そのことを伝えて交換のページへ戻す。
// メッセージの取り扱いへの有効な同意が無い (画面を開いたあとに版が変わった) ときは、そのことを伝えてメッセージの利用の画面へ送る。
// 無い交換と、交換の2人以外には、存在しないページとして404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰がやめるかを決められないままやめない。
		slog.ErrorContext(ctx, "交換をやめる操作に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tradeID, ok := tradeIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}
	// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
	// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため、下のリダイレクトでは誤検知。
	tradePath := templates.TradePath(tradeID.String())

	form := submittedForm{reason: r.PostFormValue("reason"), note: r.PostFormValue("note")}
	err := h.cancelTradeUC.Execute(ctx, usecase.CancelTradeInput{UserID: user.ID, TradeID: tradeID, Reason: form.reason, Note: form.note})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.render(w, r, user, tradeID, http.StatusUnprocessableEntity, form, ve)
			return
		}
		if ae := model.AsAppError(err); ae != nil {
			switch ae.Code {
			case model.AppErrCodeResourceNotFound:
				h.errorRenderer.NotFound(w, r)
				return
			case model.AppErrCodeConflict:
				h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_trade_cancellation_conflict"))
				//nolint:gosec // G710
				http.Redirect(w, r, tradePath, http.StatusSeeOther)
				return
			case model.AppErrCodeMessageConsentRequired:
				h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_trade_cancellation_consent_required"))
				http.Redirect(w, r, templates.SettingsMessageConsentPath, http.StatusSeeOther)
				return
			}
		}
		slog.ErrorContext(ctx, "交換をやめるのに失敗しました", "error", err, "user_id", user.ID, "trade_id", tradeID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_trade_cancelled"))
	//nolint:gosec // G710
	http.Redirect(w, r, tradePath, http.StatusSeeOther)
}
