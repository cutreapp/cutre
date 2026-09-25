package model_test

import (
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestUserSession_NeedsExtension は、期限の延長を間引く境界を検証する。
// 閲覧のたびにUPDATEが走らないことと、間隔を過ぎたら必ず延ばすことの両方が条件になる。
func TestUserSession_NeedsExtension(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		lastSeenAt time.Time
		want       bool
	}{
		{
			name:       "直前に使ったセッションは延長しない",
			lastSeenAt: now.Add(-1 * time.Minute),
			want:       false,
		},
		{
			name:       "間隔の直前は延長しない",
			lastSeenAt: now.Add(-24*time.Hour + time.Second),
			want:       false,
		},
		{
			name:       "間隔ちょうどで延長する",
			lastSeenAt: now.Add(-24 * time.Hour),
			want:       true,
		},
		{
			name:       "間隔を過ぎていれば延長する",
			lastSeenAt: now.Add(-72 * time.Hour),
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			session := &model.UserSession{LastSeenAt: tt.lastSeenAt}

			if got := session.NeedsExtension(now); got != tt.want {
				t.Errorf("NeedsExtension() = %t、期待値 = %t", got, tt.want)
			}
		})
	}
}

// TestUserSessionExpiresAt は、有効期限が起点から有効期間ぶん先になることを検証する。
// ログイン時と延長時で別々の長さにならないよう、期限の計算はこの関数に集約している。
func TestUserSessionExpiresAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	want := now.Add(model.UserSessionLifetime)
	if got := model.UserSessionExpiresAt(now); !got.Equal(want) {
		t.Errorf("UserSessionExpiresAt() = %v、期待値 = %v", got, want)
	}
}
