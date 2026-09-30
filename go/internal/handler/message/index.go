package message

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/message"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Index GET /messages - ユーザーの交換ごとのメッセージを、最新のメッセージが新しい順に描画する。メインメニューのメッセージの行き先。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のメッセージかを決められないまま描画しない。
		slog.ErrorContext(ctx, "メッセージの一覧に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	output, err := h.getMessagesUC.Execute(ctx, usecase.GetMessagesInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "メッセージの一覧の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// メッセージの日時はユーザーのタイムゾーンで示す。
	loc, err := user.Location()
	if err != nil {
		slog.ErrorContext(ctx, "ユーザーのタイムゾーンの読み込みに失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "message_index_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMessage, CurrentPath: templates.MessagesPath},
	}
	data := page.IndexPageData{
		Rows: viewmodel.NewTradeMessageListRows(
			ctx, user.ID, output.Trades, output.Partners, output.Items, output.LatestMessages, output.UnreadCounts, loc, time.Now(),
		),
	}
	if err := layouts.Default(layoutData, page.Index(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "メッセージの一覧の描画に失敗しました", "error", err)
	}
}
