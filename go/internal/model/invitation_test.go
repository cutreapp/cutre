package model_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestInvitation_IsUsable は、未取り消し・期限内で、人数に空きがある招待だけを使えると判定することを検証する。
// 人数の上限は、管理者の招待では1人、ユーザーの招待では InviterRedemptionLimit になる。
func TestInvitation_IsUsable(t *testing.T) {
	t.Parallel()

	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)
	inviterID := model.UserID(uuid.New())

	tests := []struct {
		name          string
		invitation    model.Invitation
		redeemedCount int
		want          bool
	}{
		{name: "管理者の招待・未使用", invitation: model.Invitation{ExpiresAt: future}, want: true},
		{name: "管理者の招待・使用済み", invitation: model.Invitation{ExpiresAt: future}, redeemedCount: 1, want: false},
		{name: "ユーザーの招待・上限の1人手前", invitation: model.Invitation{InviterUserID: &inviterID, ExpiresAt: future}, redeemedCount: model.InviterRedemptionLimit - 1, want: true},
		{name: "ユーザーの招待・上限に達した", invitation: model.Invitation{InviterUserID: &inviterID, ExpiresAt: future}, redeemedCount: model.InviterRedemptionLimit, want: false},
		{name: "期限ちょうど", invitation: model.Invitation{ExpiresAt: now}, want: false},
		{name: "期限切れ", invitation: model.Invitation{ExpiresAt: past}, want: false},
		{name: "取り消し済み", invitation: model.Invitation{ExpiresAt: future, RevokedAt: &past}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.invitation.IsUsable(now, tt.redeemedCount); got != tt.want {
				t.Errorf("IsUsable() = %t、期待値 = %t", got, tt.want)
			}
		})
	}
}

// TestUserInvitationExpiresAt は、招待者のタイムゾーンで発行日から7日後の日の終わりを期限にし、
// その日付を最後に使える日として返すことを検証する。
func TestUserInvitationExpiresAt(t *testing.T) {
	t.Parallel()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("タイムゾーンの読み込みのエラー = %v", err)
	}
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("タイムゾーンの読み込みのエラー = %v", err)
	}

	tests := []struct {
		name     string
		now      time.Time
		loc      *time.Location
		want     time.Time
		wantDate string
	}{
		{
			name:     "日本時間の昼に発行",
			now:      time.Date(2026, 9, 25, 12, 0, 0, 0, tokyo),
			loc:      tokyo,
			want:     time.Date(2026, 10, 3, 0, 0, 0, 0, tokyo),
			wantDate: "2026-10-02",
		},
		{
			name:     "日本時間の日付が変わる直前に発行",
			now:      time.Date(2026, 9, 25, 23, 59, 0, 0, tokyo),
			loc:      tokyo,
			want:     time.Date(2026, 10, 3, 0, 0, 0, 0, tokyo),
			wantDate: "2026-10-02",
		},
		{
			// UTCでは9月26日でも、招待者のタイムゾーンでは9月25日の発行になる。
			name:     "招待者のタイムゾーンの日付で数える",
			now:      time.Date(2026, 9, 26, 2, 0, 0, 0, time.UTC),
			loc:      newYork,
			want:     time.Date(2026, 10, 3, 0, 0, 0, 0, newYork),
			wantDate: "2026-10-02",
		},
		{
			// 期間中に夏時間が終わっても、期限は現地の日の境目に揃う。
			name:     "期間中に夏時間が終わる",
			now:      time.Date(2026, 10, 30, 12, 0, 0, 0, newYork),
			loc:      newYork,
			want:     time.Date(2026, 11, 7, 0, 0, 0, 0, newYork),
			wantDate: "2026-11-06",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := model.UserInvitationExpiresAt(tt.now, tt.loc)
			if !got.Equal(tt.want) {
				t.Errorf("UserInvitationExpiresAt() = %v、期待値 = %v", got, tt.want)
			}

			invitation := model.Invitation{ExpiresAt: got}
			if date := invitation.LastUsableDate(tt.loc).Format(time.DateOnly); date != tt.wantDate {
				t.Errorf("LastUsableDate() = %s、期待値 = %s", date, tt.wantDate)
			}
		})
	}
}

// TestInviterRemainingRedemptions は、これから登録できる人数を上限から数え、0を下回らないことを検証する。
func TestInviterRemainingRedemptions(t *testing.T) {
	t.Parallel()

	for redeemed, want := range map[int]int{0: model.InviterRedemptionLimit, 2: model.InviterRedemptionLimit - 2, model.InviterRedemptionLimit: 0, model.InviterRedemptionLimit + 1: 0} {
		if got := model.InviterRemainingRedemptions(redeemed); got != want {
			t.Errorf("InviterRemainingRedemptions(%d) = %d、期待値 = %d", redeemed, got, want)
		}
	}
}
