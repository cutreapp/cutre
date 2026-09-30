package trade

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/trade"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /@{atname}/trades/new - アットネーム atname のユーザーに交換を申し込む組み合わせを選ぶ画面を描画する。
//
// 申し込み内容の確認から組み合わせを変えに戻ったときは、クエリの receive_item_ids・give_item_ids のアイテムを選んだ状態で出す。
// 読めないIDは無視する。メッセージの取り扱いへの有効な同意が無ければ、選ぶ代わりに同意の案内を出す。
// いないか退会したユーザーと、自分のアットネームは存在しないページとして扱う。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰が申し込むかを決められないまま描画しない。
		slog.ErrorContext(ctx, "交換の組み合わせを選ぶ画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	output, err := h.getTradeProposalUC.Execute(ctx, usecase.GetTradeProposalInput{ProposerUserID: user.ID, Atname: chi.URLParam(r, "atname")})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "交換の組み合わせの選択肢の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	receiveIDs, _ := model.ParseItemIDs(r.URL.Query()["receive_item_ids"])
	giveIDs, _ := model.ParseItemIDs(r.URL.Query()["give_item_ids"])

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "trade_new_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// マッチ候補と相手のプロフィールと同じく、交換から辿る画面としてメインメニューの交換の中に置く。
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavTrade, CurrentPath: templates.NewTradePath(output.Receiver.Atname)},
	}
	data := page.NewPageData{
		Proposal:            viewmodel.NewTradeProposal(output.Receiver.Atname, output.Match, output.Goods, output.EventCategories, receiveIDs, giveIDs),
		MessageConsentValid: output.MessageConsentValid,
	}
	if err := layouts.Default(layoutData, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "交換の組み合わせを選ぶ画面の描画に失敗しました", "error", err)
	}
}
