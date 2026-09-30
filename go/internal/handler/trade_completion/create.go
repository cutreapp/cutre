package trade_completion

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /trades/{id}/completion - 「交換できた」を記録し、完了のメッセージを付けて交換のページへ戻す。
// 相手がすでに押していれば交換が終わり、そのことをメッセージで伝える。
//
// ひとことを受け付けなかったときは、送られた値とエラーを付けて「交換できた」の画面を描き直す (422)。
// 画面を開いたあとに交換が終わっていた (別のタブで先に押していた) ときは、記録せずに、そのことを伝えて交換のページへ戻す。
// ひとことを入れたのに、メッセージの取り扱いへの有効な同意が無いときは、そのことを伝えてメッセージの利用の画面へ送る。
// 無い交換と、交換の2人以外には、存在しないページとして404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰が押したかを決められないまま記録しない。
		slog.ErrorContext(ctx, "「交換できた」の記録に現在のユーザーがありません (RequireAuth を通していません)")
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

	note := r.PostFormValue("note")
	output, err := h.completeTradeUC.Execute(ctx, usecase.CompleteTradeInput{UserID: user.ID, TradeID: tradeID, Note: note})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.render(w, r, user, tradeID, http.StatusUnprocessableEntity, note, ve)
			return
		}
		if ae := model.AsAppError(err); ae != nil {
			switch ae.Code {
			case model.AppErrCodeResourceNotFound:
				h.errorRenderer.NotFound(w, r)
				return
			case model.AppErrCodeConflict:
				h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_trade_completion_conflict"))
				//nolint:gosec // G710
				http.Redirect(w, r, tradePath, http.StatusSeeOther)
				return
			case model.AppErrCodeMessageConsentRequired:
				h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_trade_completion_consent_required"))
				http.Redirect(w, r, templates.SettingsMessageConsentPath, http.StatusSeeOther)
				return
			}
		}
		slog.ErrorContext(ctx, "「交換できた」の記録に失敗しました", "error", err, "user_id", user.ID, "trade_id", tradeID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if output.Completed {
		h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_trade_completed"))
	} else {
		h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_trade_completion_recorded"))
	}
	//nolint:gosec // G710
	http.Redirect(w, r, tradePath, http.StatusSeeOther)
}
