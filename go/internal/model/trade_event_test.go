package model_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestParseTradeReasons は、お断り・交換できなかった・やめるの理由を、それぞれの選択肢の中からだけ読むことを検証する。
func TestParseTradeReasons(t *testing.T) {
	t.Parallel()

	if reason, ok := model.ParseTradeDeclineReason("place_mismatch"); !ok || reason != model.TradeDeclineReasonPlaceMismatch {
		t.Errorf("ParseTradeDeclineReason(place_mismatch) = (%q, %v)、交換場所が合わないを期待", reason, ok)
	}
	if reason, ok := model.ParseTradeFailureReason("no_show"); !ok || reason != model.TradeFailureReasonNoShow {
		t.Errorf("ParseTradeFailureReason(no_show) = (%q, %v)、当日会えなかったを期待", reason, ok)
	}
	if reason, ok := model.ParseTradeCancellationReason("item_unavailable"); !ok || reason != model.TradeCancellationReasonItemUnavailable {
		t.Errorf("ParseTradeCancellationReason(item_unavailable) = (%q, %v)、品物が手元になくなったを期待", reason, ok)
	}

	// ほかの操作の理由と空は、選択肢に無いものとして読まない。
	if reason, ok := model.ParseTradeFailureReason("place_mismatch"); ok {
		t.Errorf("ParseTradeFailureReason(place_mismatch) = (%q, true)、読まないことを期待", reason)
	}
	if reason, ok := model.ParseTradeCancellationReason("no_show"); ok {
		t.Errorf("ParseTradeCancellationReason(no_show) = (%q, true)、読まないことを期待", reason)
	}
	if reason, ok := model.ParseTradeCancellationReason(""); ok {
		t.Errorf("ParseTradeCancellationReason(\"\") = (%q, true)、読まないことを期待", reason)
	}
}
