package ratelimit

import (
	"testing"
	"time"
)

// TestStartOfWindow は同じ長さの固定ウィンドウが境界で切り替わることを検証する。
func TestStartOfWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "境界の直前は現在の時間枠に属する",
			now:  time.Date(2026, time.September, 21, 0, 59, 59, 0, time.UTC),
			want: time.Date(2026, time.September, 21, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "境界で次の時間枠へ切り替わる",
			now:  time.Date(2026, time.September, 21, 1, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.September, 21, 1, 0, 0, 0, time.UTC),
		},
		{
			name: "UTC以外の時刻もUTCの時間枠へ揃える",
			now:  time.Date(2026, time.September, 21, 10, 30, 0, 0, time.FixedZone("JST", 9*60*60)),
			want: time.Date(2026, time.September, 21, 1, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := startOfWindow(tt.now, time.Hour); !got.Equal(tt.want) {
				t.Errorf("startOfWindow() = %v、期待値 = %v", got, tt.want)
			}
		})
	}
}
