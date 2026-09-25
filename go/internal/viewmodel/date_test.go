package viewmodel_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestFormatDate は、表示中のロケールの書式で日付を返し、今年の日付だけ年を省くことを検証する。
func TestFormatDate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		lang string
		date time.Time
		want string
	}{
		{name: "日本語・今年", lang: i18n.LangJa, date: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), want: "10月2日"},
		{name: "日本語・別の年", lang: i18n.LangJa, date: time.Date(2027, 1, 3, 0, 0, 0, 0, time.UTC), want: "2027年1月3日"},
		{name: "英語・今年", lang: i18n.LangEn, date: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), want: "Oct 2"},
		{name: "英語・別の年", lang: i18n.LangEn, date: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), want: "Dec 31, 2025"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.lang)
			if got := viewmodel.FormatDate(ctx, tt.date, now); got != tt.want {
				t.Errorf("FormatDate() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}
