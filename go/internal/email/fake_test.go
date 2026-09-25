package email_test

import (
	"github.com/cutreapp/cutre/go/internal/email"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// テストダブルが Sender を満たし続けていることを、Senderを変えた時点で検出する。
var _ email.Sender = (*testutil.FakeEmailSender)(nil)
