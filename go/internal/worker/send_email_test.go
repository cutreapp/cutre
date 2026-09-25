package worker_test

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/email"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/worker"
)

// TestSendEmailConfirmationWorker_Work は、ジョブの引数の宛先・コード・言語でメールを送り、
// ジョブごとに決まるIdempotencyKeyを添えることを検証する。
func TestSendEmailConfirmationWorker_Work(t *testing.T) {
	t.Parallel()

	fake := &testutil.FakeEmailSender{}
	w := worker.NewSendEmailConfirmationWorker(usecase.NewSendEmailConfirmationUsecase(email.NewConfirmationSender(fake)))

	job := &river.Job[dispatcher.SendEmailConfirmationArgs]{
		JobRow: &rivertype.JobRow{ID: 42, Kind: "send_email_confirmation"},
		Args:   dispatcher.SendEmailConfirmationArgs{Email: "user@example.com", Code: "012345", Locale: model.LocaleEn},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	if len(fake.Sent) != 1 {
		t.Fatalf("送ったメールの件数 = %d、期待値 = 1", len(fake.Sent))
	}
	sent := fake.Sent[0]
	if sent.To != "user@example.com" || sent.Subject != "Your Cutre confirmation code" || sent.IdempotencyKey != "send_email_confirmation/42" {
		t.Errorf("送ったメール = (%q, %q, %q)、宛先・英語の件名・ジョブのIDによるキーを期待", sent.To, sent.Subject, sent.IdempotencyKey)
	}
}

// TestSendAlreadyRegisteredNoticeWorker_Work は、ジョブの引数の宛先と言語でログインを案内するメールを送ることを検証する。
func TestSendAlreadyRegisteredNoticeWorker_Work(t *testing.T) {
	t.Parallel()

	fake := &testutil.FakeEmailSender{}
	w := worker.NewSendAlreadyRegisteredNoticeWorker(usecase.NewSendAlreadyRegisteredNoticeUsecase(
		email.NewAlreadyRegisteredNoticeSender(fake, "https://cutre.example.com"),
	))

	job := &river.Job[dispatcher.SendAlreadyRegisteredNoticeArgs]{
		JobRow: &rivertype.JobRow{ID: 7, Kind: "send_already_registered_notice"},
		Args:   dispatcher.SendAlreadyRegisteredNoticeArgs{Email: "user@example.com", Locale: model.LocaleJa},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	if len(fake.Sent) != 1 {
		t.Fatalf("送ったメールの件数 = %d、期待値 = 1", len(fake.Sent))
	}
	sent := fake.Sent[0]
	if sent.To != "user@example.com" || sent.Subject != "Cutreのアカウントは登録済みです" || sent.IdempotencyKey != "send_already_registered_notice/7" {
		t.Errorf("送ったメール = (%q, %q, %q)、宛先・日本語の件名・ジョブのIDによるキーを期待", sent.To, sent.Subject, sent.IdempotencyKey)
	}
}

// TestSendPasswordResetWorker_Work は、ジョブの引数のユーザーのアドレスへ、引数の言語でパスワードリセットのメールを送ることを検証する。
func TestSendPasswordResetWorker_Work(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	userEmail := testutil.UniqueEmail("send-password-reset-worker")
	userID := testutil.NewUserBuilder(t, db).WithEmail(userEmail).Build()

	fake := &testutil.FakeEmailSender{}
	w := worker.NewSendPasswordResetWorker(usecase.NewSendPasswordResetUsecase(
		db,
		repository.NewUserRepository(db),
		repository.NewPasswordResetTokenRepository(db),
		email.NewPasswordResetSender(fake, "https://cutre.example.com"),
	))

	job := &river.Job[dispatcher.SendPasswordResetArgs]{
		JobRow: &rivertype.JobRow{ID: 9, Kind: "send_password_reset"},
		Args:   dispatcher.SendPasswordResetArgs{UserID: userID, Locale: model.LocaleEn},
	}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	if len(fake.Sent) != 1 {
		t.Fatalf("送ったメールの件数 = %d、期待値 = 1", len(fake.Sent))
	}
	sent := fake.Sent[0]
	if sent.To != userEmail || sent.Subject != "Reset your Cutre password" || sent.IdempotencyKey != "" {
		t.Errorf("送ったメール = (%q, %q, %q)、ユーザーの宛先・英語の件名・空のキーを期待", sent.To, sent.Subject, sent.IdempotencyKey)
	}
}
