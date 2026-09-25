package settings_invitation

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/qrcode"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/settings_invitation"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET /settings/invitation - 招待リンクと参加した人を描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
//
// 使える招待が無く人数に空きがあれば、描画の前に招待を作る。GETで行を作ることになるが、
// ログイン後の no-store の画面でプリフェッチやクローラーが届かず、何度開いても「使える招待が1本ある」状態に収束するため許容する。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の招待かを決められないまま描画しない。
		slog.ErrorContext(ctx, "招待の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data, err := h.pageData(r, user)
	if err != nil {
		slog.ErrorContext(ctx, "招待の画面の準備に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data.CSRFToken = middleware.CSRFTokenFromContext(ctx)

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "settings_invitation_show_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:   meta,
		Header: &components.HeaderData{Atname: user.Atname, CurrentPath: templates.SettingsInvitationPath},
	}
	if err := layouts.Default(layoutData, page.Show(*data)).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "招待の画面の描画に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// pageData は、招待を用意して参加した人を引き、画面に出す形にする。
//
// 日付はユーザーのタイムゾーンで示す。招待の期限もそのタイムゾーンの日の終わりに揃えているため。
func (h *Handler) pageData(r *http.Request, user *model.User) (*page.ShowPageData, error) {
	ctx := r.Context()

	prepared, err := h.prepareInvitationUC.Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
	if err != nil {
		return nil, err
	}
	redemptions, err := h.getInvitationRedemptionsUC.Execute(ctx, usecase.GetInvitationRedemptionsInput{InviterUserID: user.ID})
	if err != nil {
		return nil, err
	}
	loc, err := user.Location()
	if err != nil {
		return nil, err
	}

	return h.showPageData(ctx, user, prepared.Invitation, redemptions.Redemptions, loc, time.Now())
}

// showPageData は、用意した招待と参加した人から画面に出すデータを組む。
//
// 残りの人数とリンクを出すかは、参加した人の一覧の件数から決める。招待を用意してから一覧を引くまでに
// 最後の枠が埋まっても、上限に達した一覧と招待リンクを同時に出さないため。
func (h *Handler) showPageData(
	ctx context.Context,
	user *model.User,
	invitation *model.Invitation,
	redemptions []*model.InvitationRedemption,
	loc *time.Location,
	now time.Time,
) (*page.ShowPageData, error) {
	data := &page.ShowPageData{
		ProfilePath:    templates.ProfilePath(user.Atname),
		RemainingCount: model.InviterRemainingRedemptions(len(redemptions)),
		Redemptions:    viewmodel.NewInvitationRedemptions(ctx, redemptions, loc, now),
	}
	if invitation == nil || data.RemainingCount == 0 {
		return data, nil
	}

	// 招待される人の言語は分からないため、言語コードを持たない日本語版のURLにする。
	// 受け取りの画面は言語を切り替えられ、URLが短いほどQRコードも粗く読み取りやすくなる。
	url := h.cfg.AppURL() + templates.InvitationPath(invitation.Token)
	code, err := qrcode.Encode(url)
	if err != nil {
		return nil, err
	}
	userInvitation := viewmodel.NewUserInvitation(ctx, invitation, url, loc, now)
	data.Invitation = &userInvitation
	data.QRCode = code

	return data, nil
}
