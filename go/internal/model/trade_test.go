package model_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestTrade_Participants は、交換の2人を見分け、2人のうち相手を返すことを検証する。
func TestTrade_Participants(t *testing.T) {
	t.Parallel()

	proposer := model.UserID(uuid.New())
	receiver := model.UserID(uuid.New())
	other := model.UserID(uuid.New())
	trade := &model.Trade{ProposerUserID: proposer, ReceiverUserID: receiver}

	if !trade.IsParticipant(proposer) || !trade.IsParticipant(receiver) {
		t.Error("交換の2人を IsParticipant = true と判定することを期待したが、falseだった")
	}
	if trade.IsParticipant(other) {
		t.Error("交換の2人以外を IsParticipant = false と判定することを期待したが、trueだった")
	}
	if !trade.IsProposer(proposer) || trade.IsProposer(receiver) {
		t.Error("申し込んだ人だけを IsProposer = true と判定することを期待した")
	}
	if got := trade.PartnerUserID(proposer); got != receiver {
		t.Errorf("申し込んだ人の相手 = %v、期待値 = %v", got, receiver)
	}
	if got := trade.PartnerUserID(receiver); got != proposer {
		t.Errorf("申し込まれた人の相手 = %v、期待値 = %v", got, proposer)
	}
}

// TestTrade_IsAwaiting は、申し込まれた人の返事待ちと、相手だけが「交換できた」を押したマッチ成立を、
// その人の返事や確認を待っている交換と判定することを検証する。
func TestTrade_IsAwaiting(t *testing.T) {
	t.Parallel()

	proposer := model.UserID(uuid.New())
	receiver := model.UserID(uuid.New())
	completedAt := time.Now()

	tests := []struct {
		name         string
		trade        model.Trade
		wantProposer bool
		wantReceiver bool
	}{
		{name: "返事待ちは申し込まれた人を待つ", trade: model.Trade{Status: model.TradeStatusPending}, wantReceiver: true},
		{name: "どちらも押していないマッチ成立は誰も待たない", trade: model.Trade{Status: model.TradeStatusMatched}},
		{
			name:         "申し込まれた人だけが押したマッチ成立は申し込んだ人を待つ",
			trade:        model.Trade{Status: model.TradeStatusMatched, ReceiverCompletedAt: &completedAt},
			wantProposer: true,
		},
		{
			name:         "申し込んだ人だけが押したマッチ成立は申し込まれた人を待つ",
			trade:        model.Trade{Status: model.TradeStatusMatched, ProposerCompletedAt: &completedAt},
			wantReceiver: true,
		},
		{name: "取り下げた交換は誰も待たない", trade: model.Trade{Status: model.TradeStatusWithdrawn}},
		{
			name:  "終わった交換は誰も待たない",
			trade: model.Trade{Status: model.TradeStatusCompleted, ProposerCompletedAt: &completedAt, ReceiverCompletedAt: &completedAt},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			trade := tt.trade
			trade.ProposerUserID = proposer
			trade.ReceiverUserID = receiver
			if got := trade.IsAwaiting(proposer); got != tt.wantProposer {
				t.Errorf("申し込んだ人の IsAwaiting = %v、期待値 = %v", got, tt.wantProposer)
			}
			if got := trade.IsAwaiting(receiver); got != tt.wantReceiver {
				t.Errorf("申し込まれた人の IsAwaiting = %v、期待値 = %v", got, tt.wantReceiver)
			}
			if trade.IsAwaiting(model.UserID(uuid.New())) {
				t.Error("交換の2人以外を IsAwaiting = false と判定することを期待したが、trueだった")
			}
		})
	}
}

// TestTrade_IsInProgress は、返事待ちとマッチ成立だけを進行中と判定することを検証する。
func TestTrade_IsInProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status model.TradeStatus
		want   bool
	}{
		{status: model.TradeStatusPending, want: true},
		{status: model.TradeStatusMatched, want: true},
		{status: model.TradeStatusWithdrawn},
		{status: model.TradeStatusDeclined},
		{status: model.TradeStatusCompleted},
		{status: model.TradeStatusFailed},
		{status: model.TradeStatusCancelled},
	}
	for _, tt := range tests {
		if got := (&model.Trade{Status: tt.status}).IsInProgress(); got != tt.want {
			t.Errorf("%s: IsInProgress() = %v、期待値 = %v", tt.status, got, tt.want)
		}
	}
}

// TestTrade_HasCompleted は、2人それぞれが「交換できた」を押したかを、その人の押した時刻で判定することを検証する。
func TestTrade_HasCompleted(t *testing.T) {
	t.Parallel()

	proposer := model.UserID(uuid.New())
	receiver := model.UserID(uuid.New())
	pressedAt := time.Now()
	trade := &model.Trade{ProposerUserID: proposer, ReceiverUserID: receiver, Status: model.TradeStatusMatched, ReceiverCompletedAt: &pressedAt}

	if trade.HasCompleted(proposer) {
		t.Error("押していない申し込んだ人を HasCompleted = false と判定することを期待したが、trueだった")
	}
	if !trade.HasCompleted(receiver) {
		t.Error("押した申し込まれた人を HasCompleted = true と判定することを期待したが、falseだった")
	}
}

// TestTrade_HasAnyCompleted は、2人のどちらかの押した時刻が入っていれば、どちらかが「交換できた」を押したと判定することを検証する。
func TestTrade_HasAnyCompleted(t *testing.T) {
	t.Parallel()

	pressedAt := time.Now()
	tests := []struct {
		name  string
		trade *model.Trade
		want  bool
	}{
		{name: "どちらも押していない", trade: &model.Trade{}, want: false},
		{name: "申し込んだ人だけが押した", trade: &model.Trade{ProposerCompletedAt: &pressedAt}, want: true},
		{name: "申し込まれた人だけが押した", trade: &model.Trade{ReceiverCompletedAt: &pressedAt}, want: true},
	}
	for _, tt := range tests {
		if got := tt.trade.HasAnyCompleted(); got != tt.want {
			t.Errorf("%s: HasAnyCompleted() = %v、期待値 = %v", tt.name, got, tt.want)
		}
	}
}
