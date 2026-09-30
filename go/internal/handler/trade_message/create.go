package trade_message

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /trades/{id}/messages - 交換でメッセージを送り、メッセージのページの末尾へ戻す。
//
// 本文を受け付けなかったときは、送られた本文とエラーを付けてメッセージのページを描き直す (422)。
// 送信が続いて上限を超えたときも、同じページを描き直す (429)。
// 画面を開いたあとに交換が終わっていたときは、送らずにそのことを伝えてメッセージのページへ戻す。
// メッセージの取り扱いへの有効な同意が無いときは、そのことを伝えてメッセージの利用の画面へ送る。
// 無い交換と、交換の2人以外には、存在しないページとして404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰が送るかを決められないまま送らない。
		slog.ErrorContext(ctx, "交換のメッセージの送信に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tradeID, ok := tradeIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}
	body := r.PostFormValue("body")

	resetAt, err := h.limiter.CheckTradeMessage(ctx, user.ID.String())
	if err != nil {
		slog.ErrorContext(ctx, "交換のメッセージのレート制限の判定に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		untilReset := time.Until(resetAt)
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(untilReset.Seconds()))))
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "validation_trade_message_rate_limited", map[string]any{"Minutes": ratelimit.WaitMinutes(untilReset)}))
		h.render(w, r, user, tradeID, http.StatusTooManyRequests, body, ve)
		return
	}

	// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
	// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため、下のリダイレクトでは誤検知。
	messagesPath := templates.TradeMessagesLatestPath(tradeID.String())

	_, err = h.createTradeMessageUC.Execute(ctx, usecase.CreateTradeMessageInput{SenderUserID: user.ID, TradeID: tradeID, Body: body})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.render(w, r, user, tradeID, http.StatusUnprocessableEntity, body, ve)
			return
		}
		if ae := model.AsAppError(err); ae != nil {
			switch ae.Code {
			case model.AppErrCodeResourceNotFound:
				h.errorRenderer.NotFound(w, r)
				return
			case model.AppErrCodeConflict:
				h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_trade_message_trade_ended"))
				//nolint:gosec // G710
				http.Redirect(w, r, messagesPath, http.StatusSeeOther)
				return
			case model.AppErrCodeMessageConsentRequired:
				h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_message_send_consent_required"))
				http.Redirect(w, r, templates.SettingsMessageConsentPath, http.StatusSeeOther)
				return
			}
		}
		slog.ErrorContext(ctx, "交換のメッセージの送信に失敗しました", "error", err, "user_id", user.ID, "trade_id", tradeID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	//nolint:gosec // G710
	http.Redirect(w, r, messagesPath, http.StatusSeeOther)
}
