package trade_message

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/trade"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Index GET /trades/{id}/messages - 交換のメッセージのページを描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
//
// メッセージは2人だけのものため、無い交換と同じく、交換の2人以外には存在しないページとして404を返す。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		// RequireAuth を通していない配線の誤り。見てよい人かを決められないまま描画しない。
		slog.ErrorContext(r.Context(), "交換のメッセージのページに現在のユーザーがありません (RequireAuth を通していません)")
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

// render は交換 tradeID のメッセージのページを、指定したステータスで描画する。
// 送信を受け付けなかったときは、本文の欄に送られた値 body を戻し、エラー formErrors を出す。
//
// 送信の欄を出すとき (進行中の交換で、同意があるとき) は、下に固定するメインメニューを出さない。画面の下に置く送信の欄とキーボードに重ねないため。
//
// 描画するメッセージの最新のものまでを読んだことにし、前に読んだところより後の相手のメッセージの前に未読の境目を置く。
// GETで状態を変えることになるが、変えるのは開いた本人の読んだ位置だけで、何度開いても同じ結果になる。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, user *model.User, tradeID model.TradeID, status int, body string, formErrors *model.ValidationError) {
	ctx := r.Context()

	output, err := h.getTradeMessagesUC.Execute(ctx, usecase.GetTradeMessagesInput{ViewerUserID: user.ID, TradeID: tradeID})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "交換のメッセージの取得に失敗しました", "error", err, "user_id", user.ID, "trade_id", tradeID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// メッセージの日と時刻はユーザーのタイムゾーンで示す。
	loc, err := user.Location()
	if err != nil {
		slog.ErrorContext(ctx, "ユーザーのタイムゾーンの読み込みに失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	thread := viewmodel.NewTradeMessageThread(ctx, user.ID, output.Trade, output.Partner, output.Items, output.Messages, output.Events, output.LastReadPosition, loc, time.Now())

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "trade_message_index_title")

	layoutData := layouts.DefaultLayoutData{
		Meta: meta,
		MainNav: &components.MainNavData{
			Atname:       user.Atname,
			Current:      components.MainNavMessage,
			CurrentPath:  templates.TradeMessagesPath(thread.ID),
			BottomHidden: thread.InProgress && output.MessageConsentValid,
		},
	}
	data := page.MessagesPageData{
		CSRFToken:           middleware.CSRFTokenFromContext(ctx),
		Thread:              thread,
		MessageConsentValid: output.MessageConsentValid,
		Body:                body,
		FormErrors:          formErrors,
	}
	// 先読みではユーザーがまだ見ていないため、読んだことにしない。
	shouldMark := len(output.Messages) > 0 && !strings.Contains(r.Header.Get("Sec-Purpose"), "prefetch")
	html, err := renderMessagePage(ctx, shouldMark, thread.UnreadCount, func(renderCtx context.Context) ([]byte, error) {
		var buf bytes.Buffer
		if err := layouts.Default(layoutData, page.Messages(data)).Render(renderCtx, &buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}, func() error {
		latest := output.Messages[len(output.Messages)-1]
		err := h.markTradeMessagesReadUC.Execute(ctx, usecase.MarkTradeMessagesReadInput{
			UserID: user.ID, TradeID: output.Trade.ID,
			LastReadPosition: model.TradeMessageReadPosition{CreatedAt: latest.CreatedAt, MessageID: latest.ID},
		})
		if err != nil {
			slog.ErrorContext(ctx, "交換のメッセージをどこまで読んだかの記録に失敗しました", "error", err, "user_id", user.ID, "trade_id", output.Trade.ID)
		}
		return err
	})
	if err != nil {
		slog.ErrorContext(ctx, "交換のメッセージのページの描画に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(html); err != nil {
		slog.ErrorContext(ctx, "交換のメッセージのページの送信に失敗しました", "error", err)
	}
}

// renderMessagePage は描画が成功してから既読を記録し、記録できなければ元のバッジで描き直す。
func renderMessagePage(ctx context.Context, shouldMark bool, unreadCount int, render func(context.Context) ([]byte, error), mark func() error) ([]byte, error) {
	if !shouldMark {
		return render(ctx)
	}
	badges := templates.MainNavBadgesFromContext(ctx)
	badges.UnreadMessageCount = max(badges.UnreadMessageCount-int64(unreadCount), 0)
	html, err := render(templates.WithMainNavBadges(ctx, badges))
	if err != nil {
		return nil, fmt.Errorf("描画に失敗: %w", err)
	}
	if err := mark(); err != nil {
		return render(ctx)
	}
	return html, nil
}
