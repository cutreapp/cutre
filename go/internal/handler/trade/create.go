package trade

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
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/trade"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Create POST /trades - アットネーム atname のユーザーに交換を申し込み、完了のメッセージを付けて交換のページへ送る。
//
// フォームを受け付けなかったときは、送られた値とエラーを付けて申し込み内容の確認の画面を描き直す (422)。
// 申し込みが続いて上限を超えたときも、同じ画面を描き直す (429)。
// メッセージの取り扱いへの有効な同意が無いとき (画面を開いたあとに別の画面でやめた) は、そのことを伝えてメッセージの利用の画面へ送る。
// いないか退会したユーザーと、自分のアットネームは存在しないページとして扱う。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰が申し込むかを決められないまま申し込まない。
		slog.ErrorContext(ctx, "交換の申し込みに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	atname := r.PostFormValue("atname")
	form := submittedForm{
		atname:         atname,
		receiveItemIDs: r.PostForm["receive_item_ids"],
		giveItemIDs:    r.PostForm["give_item_ids"],
		note:           r.PostFormValue("note"),
	}

	resetAt, err := h.limiter.CheckTradeProposal(ctx, user.ID.String())
	if err != nil {
		slog.ErrorContext(ctx, "交換の申し込みのレート制限の判定に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		untilReset := time.Until(resetAt)
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(untilReset.Seconds()))))
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "validation_trade_rate_limited", map[string]any{"Minutes": ratelimit.WaitMinutes(untilReset)}))
		h.renderConfirmation(w, r, user, http.StatusTooManyRequests, form, ve)
		return
	}

	output, err := h.createTradeUC.Execute(ctx, usecase.CreateTradeInput{
		ProposerUserID: user.ID,
		Atname:         atname,
		ReceiveItemIDs: form.receiveItemIDs,
		GiveItemIDs:    form.giveItemIDs,
		Note:           form.note,
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.renderConfirmation(w, r, user, http.StatusUnprocessableEntity, form, ve)
			return
		}
		if ae := model.AsAppError(err); ae != nil {
			switch ae.Code {
			case model.AppErrCodeResourceNotFound:
				h.errorRenderer.NotFound(w, r)
				return
			case model.AppErrCodeMessageConsentRequired:
				h.flashMgr.SetWarning(w, i18n.T(ctx, "flash_trade_message_consent_required"))
				http.Redirect(w, r, templates.SettingsMessageConsentPath, http.StatusSeeOther)
				return
			}
		}
		slog.ErrorContext(ctx, "交換の申し込みに失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_trade_created", map[string]any{"Atname": output.Receiver.Atname}))
	http.Redirect(w, r, templates.TradePath(output.Trade.ID.String()), http.StatusSeeOther)
}

// submittedForm は申し込みのフォームで送られた値。
type submittedForm struct {
	atname         string
	receiveItemIDs []string
	giveItemIDs    []string
	note           string
}

// renderConfirmation は、申し込みを受け付けなかったときに、送られた組み合わせとひとことを戻して申し込み内容の確認の画面を指定したステータスで描き直す。
// 組み合わせは今も交換できるアイテムだけを出す。リストから外れたアイテムはエラーで選び直しを案内する。
func (h *Handler) renderConfirmation(w http.ResponseWriter, r *http.Request, user *model.User, status int, form submittedForm, formErrors *model.ValidationError) {
	ctx := r.Context()

	output, err := h.getTradeProposalUC.Execute(ctx, usecase.GetTradeProposalInput{ProposerUserID: user.ID, Atname: form.atname})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "交換の組み合わせの選択肢の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	receiveIDs, _ := model.ParseItemIDs(form.receiveItemIDs)
	giveIDs, _ := model.ParseItemIDs(form.giveItemIDs)
	selected := output.Match.Select(receiveIDs, giveIDs)

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "trade_confirmation_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavTrade, CurrentPath: templates.NewTradeConfirmationPath(output.Receiver.Atname)},
	}
	data := page.ConfirmationPageData{
		CSRFToken:           middleware.CSRFTokenFromContext(ctx),
		Proposal:            viewmodel.NewTradeProposal(output.Receiver.Atname, selected, output.Goods, output.EventCategories, receiveIDs, giveIDs),
		Note:                form.note,
		MessageConsentValid: output.MessageConsentValid,
		FormErrors:          formErrors,
	}
	if err := layouts.Default(layoutData, page.Confirmation(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "申し込み内容の確認の画面の描画に失敗しました", "error", err)
	}
}
