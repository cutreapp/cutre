package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
)

// AlreadyRegisteredNoticeSender は、登録済みのメールアドレスへログインを案内するメールを送る。
type AlreadyRegisteredNoticeSender interface {
	Send(ctx context.Context, to string, locale model.Locale, idempotencyKey string) error
}

// SendAlreadyRegisteredNoticeUsecase は、登録済みのメールアドレスへログインを案内するメールを送る。ジョブのワーカーから呼ばれる。
type SendAlreadyRegisteredNoticeUsecase struct {
	sender AlreadyRegisteredNoticeSender
}

// NewSendAlreadyRegisteredNoticeUsecase は SendAlreadyRegisteredNoticeUsecase を生成する。
func NewSendAlreadyRegisteredNoticeUsecase(sender AlreadyRegisteredNoticeSender) *SendAlreadyRegisteredNoticeUsecase {
	return &SendAlreadyRegisteredNoticeUsecase{sender: sender}
}

// SendAlreadyRegisteredNoticeInput は SendAlreadyRegisteredNoticeUsecase.Execute の入力。
type SendAlreadyRegisteredNoticeInput struct {
	Email  string
	Locale model.Locale
	// IdempotencyKey は再試行で同じメールを二重に送らないためのキー。ワーカーがジョブごとに決める。
	IdempotencyKey string
}

// Execute はログインを案内するメールを送る。失敗はそのまま返し、ジョブの再試行に任せる。
func (uc *SendAlreadyRegisteredNoticeUsecase) Execute(ctx context.Context, input SendAlreadyRegisteredNoticeInput) error {
	if err := uc.sender.Send(ctx, input.Email, input.Locale, input.IdempotencyKey); err != nil {
		return fmt.Errorf("登録済みの案内のメールの送信に失敗: %w", err)
	}

	return nil
}
