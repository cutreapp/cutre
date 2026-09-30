package policy_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
)

// TestTradePolicy は、交換のページを交換の2人に、取り下げを申し込んだ人だけに、返事を申し込まれた人だけに許すことを検証する。
func TestTradePolicy(t *testing.T) {
	t.Parallel()

	proposer := model.UserID(uuid.New())
	receiver := model.UserID(uuid.New())
	trade := &model.Trade{ProposerUserID: proposer, ReceiverUserID: receiver, Status: model.TradeStatusPending}

	tests := []struct {
		name         string
		userID       model.UserID
		wantView     bool
		wantWithdraw bool
		wantReply    bool
	}{
		{name: "申し込んだ人", userID: proposer, wantView: true, wantWithdraw: true, wantReply: false},
		{name: "申し込まれた人", userID: receiver, wantView: true, wantWithdraw: false, wantReply: true},
		{name: "交換の2人以外", userID: model.UserID(uuid.New()), wantView: false, wantWithdraw: false, wantReply: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := policy.NewTradePolicy(tt.userID, trade)
			if got := p.CanView(); got != tt.wantView {
				t.Errorf("CanView() = %v、期待値 = %v", got, tt.wantView)
			}
			if got := p.CanWithdraw(); got != tt.wantWithdraw {
				t.Errorf("CanWithdraw() = %v、期待値 = %v", got, tt.wantWithdraw)
			}
			if got := p.CanReply(); got != tt.wantReply {
				t.Errorf("CanReply() = %v、期待値 = %v", got, tt.wantReply)
			}
		})
	}
}

// TestTradePolicy_CanRetractMessage は、メッセージの取り消しを、その交換のメッセージを送った人だけに、交換が終わったあとも許すことを検証する。
func TestTradePolicy_CanRetractMessage(t *testing.T) {
	t.Parallel()

	proposer := model.UserID(uuid.New())
	receiver := model.UserID(uuid.New())
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: proposer, ReceiverUserID: receiver, Status: model.TradeStatusCompleted}
	message := &model.TradeMessage{TradeID: trade.ID, SenderUserID: proposer}

	tests := []struct {
		name    string
		userID  model.UserID
		message *model.TradeMessage
		want    bool
	}{
		{name: "送った人", userID: proposer, message: message, want: true},
		{name: "交換の相手", userID: receiver, message: message, want: false},
		{name: "交換の2人以外", userID: model.UserID(uuid.New()), message: message, want: false},
		{name: "別の交換のメッセージ", userID: proposer, message: &model.TradeMessage{TradeID: model.TradeID(uuid.New()), SenderUserID: proposer}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := policy.NewTradePolicy(tt.userID, trade).CanRetractMessage(tt.message); got != tt.want {
				t.Errorf("CanRetractMessage() = %v、期待値 = %v", got, tt.want)
			}
		})
	}
}
