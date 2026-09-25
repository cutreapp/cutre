package viewmodel_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestInviterLabel は、招待者の有無と退会の状態に応じて招待した人の表示名を決めることを検証する。
func TestInviterLabel(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	inviterID := model.UserID(uuid.New())

	tests := []struct {
		name       string
		invitation *model.Invitation
		inviter    *model.User
		want       string
	}{
		{name: "管理者が発行した招待", invitation: &model.Invitation{}, want: "Cutreの運営"},
		{name: "招待した人が退会した招待", invitation: &model.Invitation{InviterUserID: &inviterID}, want: "退会したユーザー"},
		{name: "ユーザーが発行した招待", invitation: &model.Invitation{InviterUserID: &inviterID}, inviter: &model.User{Atname: "cutre_user"}, want: "@cutre_user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := viewmodel.InviterLabel(ctx, tt.invitation, tt.inviter); got != tt.want {
				t.Errorf("InviterLabel() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestNewUserInvitation は、招待を使える最後の日を招待者のタイムゾーンの日付で示すことを検証する。
func TestNewUserInvitation(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("タイムゾーンの読み込みのエラー = %v", err)
	}
	invitation := &model.Invitation{ExpiresAt: time.Date(2026, 10, 3, 0, 0, 0, 0, tokyo)}

	got := viewmodel.NewUserInvitation(ctx, invitation, "https://cutre.example.com/i/token", tokyo, time.Date(2026, 9, 25, 12, 0, 0, 0, tokyo))
	if got.URL != "https://cutre.example.com/i/token" || got.ExpiresOn != "10月2日" {
		t.Errorf("NewUserInvitation() = %+v、URLと 10月2日 を期待", got)
	}
}

// TestNewInvitationRedemptions は、参加した人を @アットネーム で、退会した人を退会したユーザーとして示し、
// 参加した日を見ているユーザーのタイムゾーンの日付にすることを検証する。
func TestNewInvitationRedemptions(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("タイムゾーンの読み込みのエラー = %v", err)
	}
	deletedAt := time.Now()
	redemptions := []*model.InvitationRedemption{
		// UTCでは9月2日でも、日本時間では9月3日の参加になる。
		{User: &model.User{Atname: "cutre_user"}, CreatedAt: time.Date(2026, 9, 2, 16, 0, 0, 0, time.UTC)},
		{User: &model.User{Atname: "deleted_0123", DeletedAt: &deletedAt}, CreatedAt: time.Date(2026, 8, 20, 3, 0, 0, 0, time.UTC)},
	}

	got := viewmodel.NewInvitationRedemptions(ctx, redemptions, tokyo, time.Date(2026, 9, 25, 12, 0, 0, 0, tokyo))
	want := []viewmodel.InvitationRedemption{
		{Label: "@cutre_user", JoinedOn: "9月3日"},
		{Label: "退会したユーザー", JoinedOn: "8月20日"},
	}
	if len(got) != len(want) {
		t.Fatalf("件数 = %d、期待値 = %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d件目 = %+v、期待値 = %+v", i, got[i], want[i])
		}
	}
}
