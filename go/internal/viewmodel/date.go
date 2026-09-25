package viewmodel

import (
	"context"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
)

// FormatDate は日付を表示中のロケールの書式で返す。
//
// nowと同じ年の日付は年を省き、別の年の日付だけ年を付ける。
// 近い日付を短く示しつつ、年をまたいだ日付を取り違えないようにするため。
// tとnowには、表示するタイムゾーンに変換した時刻を渡す。
func FormatDate(ctx context.Context, t, now time.Time) string {
	if t.Year() == now.Year() {
		return t.Format(i18n.T(ctx, "date_month_day_layout"))
	}

	return t.Format(i18n.T(ctx, "date_year_month_day_layout"))
}
