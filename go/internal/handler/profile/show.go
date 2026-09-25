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
// 他の人のプロフィールはまだ描けないため、自分以外のアットネームは存在しないページとして扱う。
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

	if !strings.EqualFold(chi.URLParam(r, "atname"), user.Atname) {
		h.errorRenderer.NotFound(w, r)
		return
	}

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

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "profile_show_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:   meta,
		Header: &components.HeaderData{Atname: user.Atname, CurrentPath: templates.ProfilePath(user.Atname)},
	}
	pageData := profilepage.ShowPageData{
		CSRFToken:                middleware.CSRFTokenFromContext(ctx),
		InvitationRemainingCount: model.InviterRemainingRedemptions(joinedCount),
		InvitationJoinedCount:    joinedCount,
		TwoFactorAuthEnabled:     twoFactorAuthStatus.TwoFactorAuth != nil,
	}
	if err := layouts.Default(layoutData, profilepage.Show(pageData)).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "プロフィールの描画に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
