package trade_confirmation

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

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

// New GET /@{atname}/trades/new/confirmation - 選んだ組み合わせで申し込む内容を確かめ、ひとことを添えて申し込むフォームを描画する。
//
// 組み合わせは、組み合わせを選ぶ画面のフォームがクエリ (receive_item_ids・give_item_ids) で送る。
// 今も交換できるアイテムだけを残し、もらうものか渡すものが残らなければ、組み合わせを選ぶ画面をエラー付きで描き直す (422)。
// メッセージの取り扱いへの有効な同意が無ければ、申し込む代わりに同意の案内を出す。
// いないか退会したユーザーと、自分のアットネームは存在しないページとして扱う。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰が申し込むかを決められないまま描画しない。
		slog.ErrorContext(ctx, "申し込み内容の確認の画面に現在のユーザーがありません (RequireAuth を通していません)")
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
	selected := output.Match.Select(receiveIDs, giveIDs)
	atname := output.Receiver.Atname

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if output.MessageConsentValid && !selected.IsTradable() {
		formErrors := model.NewValidationError()
		if len(selected.Receivable) == 0 {
			formErrors.AddField("receive_item_ids", i18n.T(ctx, "validation_trade_receive_required"))
		}
		if len(selected.Givable) == 0 {
			formErrors.AddField("give_item_ids", i18n.T(ctx, "validation_trade_give_required"))
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		data := page.NewPageData{
			Proposal:            viewmodel.NewTradeProposal(atname, output.Match, output.Goods, output.EventCategories, receiveIDs, giveIDs),
			MessageConsentValid: output.MessageConsentValid,
			FormErrors:          formErrors,
		}
		if err := layouts.Default(h.layoutData(r, user, atname, "trade_new_title"), page.New(data)).Render(ctx, w); err != nil {
			// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
			slog.ErrorContext(ctx, "交換の組み合わせを選ぶ画面の描画に失敗しました", "error", err)
		}
		return
	}

	data := page.ConfirmationPageData{
		CSRFToken:           middleware.CSRFTokenFromContext(ctx),
		Proposal:            viewmodel.NewTradeProposal(atname, selected, output.Goods, output.EventCategories, receiveIDs, giveIDs),
		MessageConsentValid: output.MessageConsentValid,
	}
	if err := layouts.Default(h.layoutData(r, user, atname, "trade_confirmation_title"), page.Confirmation(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "申し込み内容の確認の画面の描画に失敗しました", "error", err)
	}
}

// layoutData は、タイトルのキー titleKey でこの画面のレイアウトのデータを組み立てる。
//
// マッチ候補と相手のプロフィールと同じく、交換から辿る画面としてメインメニューの交換の中に置く。
func (h *Handler) layoutData(r *http.Request, user *model.User, atname, titleKey string) layouts.DefaultLayoutData {
	meta := viewmodel.SignedInPageMeta(r.Context(), h.cfg)
	meta.SetTitle(r.Context(), titleKey)

	return layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavTrade, CurrentPath: templates.NewTradeConfirmationPath(atname)},
	}
}
