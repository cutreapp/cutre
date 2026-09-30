package trade_message_retraction

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

// Create POST /trades/{id}/messages/{message_id}/retraction - 自分が送ったメッセージを取り消し、完了のメッセージを付けてメッセージのページの末尾へ戻す。
//
// 取り消しても本文は消さずに残し、画面には取り消したことだけを出す。交換が終わったあとも取り消せる。
// 無い交換・無いメッセージと、交換の2人以外・相手が送ったメッセージには、存在しないページとして404を返す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰が取り消すかを決められないまま取り消さない。
		slog.ErrorContext(ctx, "メッセージの取り消しに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	parsedTradeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.errorRenderer.NotFound(w, r)
		return
	}
	parsedMessageID, err := uuid.Parse(chi.URLParam(r, "message_id"))
	if err != nil {
		h.errorRenderer.NotFound(w, r)
		return
	}
	tradeID := model.TradeID(parsedTradeID)
	messageID := model.TradeMessageID(parsedMessageID)

	err = h.retractTradeMessageUC.Execute(ctx, usecase.RetractTradeMessageInput{UserID: user.ID, TradeID: tradeID, MessageID: messageID})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && (ae.Code == model.AppErrCodeResourceNotFound || ae.Code == model.AppErrCodeForbidden) {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "メッセージの取り消しに失敗しました", "error", err, "user_id", user.ID, "trade_id", tradeID, "trade_message_id", messageID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_trade_message_retracted"))
	// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
	// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため誤検知。
	//nolint:gosec // G710
	http.Redirect(w, r, templates.TradeMessagesLatestPath(tradeID.String()), http.StatusSeeOther)
}
