package profile

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	profilepage "github.com/cutreapp/cutre/go/internal/templates/pages/profile"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET /@{atname} - プロフィールを描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
//
// 自分のアットネームならマイページを、ほかの人のアットネームならその人のプロフィールを描く。
// アットネームは大文字小文字を区別せず一意のため、比較でも区別しない。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰のプロフィールか分からないまま描画しない。
		slog.ErrorContext(ctx, "プロフィールに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	atname := chi.URLParam(r, "atname")
	if !strings.EqualFold(atname, user.Atname) {
		h.showOther(w, r, user, atname)
		return
	}

	h.showMyPage(w, r, user)
}

// showOther はアットネーム atname のほかのユーザーのプロフィールを描画する。
// いないか退会したユーザーは存在しないページとして扱う。
func (h *Handler) showOther(w http.ResponseWriter, r *http.Request, user *model.User, atname string) {
	ctx := r.Context()

	output, err := h.getProfileUC.Execute(ctx, usecase.GetProfileInput{ViewerUserID: user.ID, Atname: atname})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "プロフィールの取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "profile_other_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// マッチ候補と同じく、交換から辿る画面としてメインメニューの交換の中に置く。
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavTrade, CurrentPath: templates.ProfilePath(output.User.Atname)},
	}
	pageData := profilepage.OtherPageData{
		Atname:              output.User.Atname,
		Places:              viewmodel.NewPlaceGroups(ctx, output.Stations),
		PlaceNote:           output.User.PlaceNote,
		CompletedTradeCount: output.CompletedTradeCount,
		Match:               viewmodel.NewMatchRows(output.Match, output.Goods, output.EventCategories),
		Tradable:            output.Match.IsTradable(),
	}
	if err := layouts.Default(layoutData, profilepage.Other(pageData)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "プロフィールの描画に失敗しました", "error", err)
	}
}

// showMyPage はユーザー user のマイページ (自分のプロフィール) を描画する。
func (h *Handler) showMyPage(w http.ResponseWriter, r *http.Request, user *model.User) {
	ctx := r.Context()

	// 招待で参加した人は累計で上限までのため、人数を数えるために一覧を引いても件数は小さく収まる。
	redemptions, err := h.getInvitationRedemptionsUC.Execute(ctx, usecase.GetInvitationRedemptionsInput{InviterUserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "招待で参加した人の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	joinedCount := len(redemptions.Redemptions)

	twoFactorAuthStatus, err := h.getTwoFactorAuthStatusUC.Execute(ctx, usecase.GetTwoFactorAuthStatusInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "二要素認証の状態の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	messageConsent, err := h.getMessageConsentUC.Execute(ctx, usecase.GetMessageConsentInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "メッセージの取り扱いへの同意の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	places, err := h.getPlacesUC.Execute(ctx, usecase.GetPlacesInput{UserID: user.ID})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "交換場所の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	endedTradeCounts, err := h.getEndedTradeCountsUC.Execute(ctx, usecase.GetEndedTradeCountsInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "終わった交換の数の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "profile_show_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.ProfilePath(user.Atname)},
	}
	pageData := profilepage.ShowPageData{
		CSRFToken:                middleware.CSRFTokenFromContext(ctx),
		InvitationRemainingCount: model.InviterRemainingRedemptions(joinedCount),
		InvitationJoinedCount:    joinedCount,
		PlaceSummary:             viewmodel.PlaceSummary(places.Stations),
		MessageConsentAgreed:     messageConsent.Consent != nil && messageConsent.Consent.IsValid(),
		TwoFactorAuthEnabled:     twoFactorAuthStatus.TwoFactorAuth != nil,
		// 行を出すかを決めるだけで、管理画面を開けるかは管理画面のUseCaseが確かめる。
		CanUseAdmin:         user.IsEditorOrAbove(),
		CompletedTradeCount: endedTradeCounts.Counts[model.TradeStatusCompleted],
		HistoryCounts:       viewmodel.NewTradeHistoryCounts(endedTradeCounts.Counts),
	}
	if err := layouts.Default(layoutData, profilepage.Show(pageData)).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "プロフィールの描画に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
