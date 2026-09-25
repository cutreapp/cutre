package settings_invitation

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TestShowPageData は、残りの人数と招待リンクを出すかを、参加した人の一覧の件数から決めることを検証する。
// 招待を用意した後に最後の枠が埋まった場合も、上限に達した一覧と招待リンクを同時に出さない。
func TestShowPageData(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	tokyo, err := time.LoadLocation(model.DefaultTimeZone)
	if err != nil {
		t.Fatalf("タイムゾーンの読み込みのエラー = %v", err)
	}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, tokyo)
	user := &model.User{Atname: "cutre_user"}
	invitation := &model.Invitation{Token: "token", ExpiresAt: time.Date(2026, 10, 3, 0, 0, 0, 0, tokyo)}
	redemptions := func(count int) []*model.InvitationRedemption {
		rows := make([]*model.InvitationRedemption, count)
		for i := range rows {
			rows[i] = &model.InvitationRedemption{User: &model.User{Atname: "member"}, CreatedAt: now}
		}
		return rows
	}

	tests := []struct {
		name           string
		invitation     *model.Invitation
		redemptions    []*model.InvitationRedemption
		wantRemaining  int
		wantInvitation bool
	}{
		{name: "招待があり人数に空きがある", invitation: invitation, redemptions: redemptions(model.InviterRedemptionLimit - 1), wantRemaining: 1, wantInvitation: true},
		{name: "招待を用意した後に一覧が上限に達した", invitation: invitation, redemptions: redemptions(model.InviterRedemptionLimit), wantRemaining: 0, wantInvitation: false},
		{name: "招待が無い", invitation: nil, redemptions: redemptions(model.InviterRedemptionLimit), wantRemaining: 0, wantInvitation: false},
	}

	h := &Handler{cfg: &config.Config{Env: "dev", Domain: "cutre.example.com"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := h.showPageData(ctx, user, tt.invitation, tt.redemptions, tokyo, now)
			if err != nil {
				t.Fatalf("showPageData()のエラー = %v", err)
			}
			if data.RemainingCount != tt.wantRemaining {
				t.Errorf("残りの人数 = %d、期待値 = %d", data.RemainingCount, tt.wantRemaining)
			}
			if len(data.Redemptions) != len(tt.redemptions) {
				t.Errorf("参加した人の件数 = %d、期待値 = %d", len(data.Redemptions), len(tt.redemptions))
			}
			if got := data.Invitation != nil && data.QRCode != nil; got != tt.wantInvitation {
				t.Errorf("招待リンクとQRコードの有無 = (%+v, %v)、有無の期待値 = %v", data.Invitation, data.QRCode != nil, tt.wantInvitation)
			}
			if tt.wantInvitation && data.Invitation.URL != h.cfg.AppURL()+"/i/token" {
				t.Errorf("招待リンク = %q、/i/token で終わるURLを期待", data.Invitation.URL)
			}
		})
	}
}
