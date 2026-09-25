package testutil

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/email"
)

// FakeEmailSender は email.Sender のテストダブル。
// メールを送らずに記録し、UseCaseやワーカーのテストで、どのメールを誰に送ろうとしたかを確かめられるようにする。
type FakeEmailSender struct {
	// Sent はSendへ渡されたメールを、渡された順に持つ。
	Sent []email.SendInput

	// Err は非nilのときSendが返すerror。送信の失敗の経路を検証するのに使う。
	Err error
}

// Send はメールを記録し、決めておいた Err を返す。
func (f *FakeEmailSender) Send(_ context.Context, input email.SendInput) error {
	f.Sent = append(f.Sent, input)
	return f.Err
}
