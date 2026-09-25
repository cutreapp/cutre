package sign_in_two_factor

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
)

// TestRateLimitedError は、次に試せるまでの時間を分に切り上げて示し、解除の直前でも1分と示すことを検証する。
func TestRateLimitedError(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	for untilReset, want := range map[time.Duration]string{
		15 * time.Minute:       "試行の回数が上限に達しました。あと15分で試せます",
		61 * time.Second:       "試行の回数が上限に達しました。あと2分で試せます",
		500 * time.Millisecond: "試行の回数が上限に達しました。あと1分で試せます",
		0:                      "試行の回数が上限に達しました。あと1分で試せます",
	} {
		ve := rateLimitedError(ctx, untilReset)
		if len(ve.Global) != 1 || ve.Global[0] != want {
			t.Errorf("%s: エラー = %v、期待値 = [%s]", untilReset, ve.Global, want)
		}
	}
}
