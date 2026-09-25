package usecase_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// fakePasswordResetSender は usecase.PasswordResetSender のテストダブル。送ろうとした宛先・トークン・言語を記録する。
type fakePasswordResetSender struct {
	mu     sync.Mutex
	to     []string
	tokens []string
	locale []model.Locale
	err    error
}

func (f *fakePasswordResetSender) Send(_ context.Context, to, token string, locale model.Locale) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.to = append(f.to, to)
	f.tokens = append(f.tokens, token)
	f.locale = append(f.locale, locale)

	return f.err
}

// TestSendPasswordResetUsecase_Execute_Concurrent は、同じユーザーへの2件のジョブが並行しても、
// 行ロックによって置換が直列化され、1件のトークンだけが残ることを検証する。
func TestSendPasswordResetUsecase_Execute_Concurrent(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	userID := testutil.NewUserBuilder(t, db).Build()
	sender := &fakePasswordResetSender{}
	uc := newSendPasswordResetUsecase(sender)

	// 先にユーザーの行をロックして、2つの処理がこの行を待つことを確かめる。
	lockTx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("ロック用トランザクションの開始に失敗: %v", err)
	}
	defer func() { _ = lockTx.Rollback() }()
	var lockedID uuid.UUID
	if err := lockTx.QueryRowContext(context.Background(), "SELECT id FROM users WHERE id = $1 FOR NO KEY UPDATE", uuid.UUID(userID)).Scan(&lockedID); err != nil {
		t.Fatalf("ユーザーの行のロックに失敗: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			done <- uc.Execute(ctx, usecase.SendPasswordResetInput{UserID: userID, Locale: model.LocaleJa})
		}()
	}
	close(start)

	completedWhileLocked := false
	var earlyErr error
	select {
	case earlyErr = <-done:
		completedWhileLocked = true
	case <-time.After(100 * time.Millisecond):
	}
	if err := lockTx.Commit(); err != nil {
		t.Fatalf("ロック用トランザクションのコミットに失敗: %v", err)
	}
	completed := 0
	if completedWhileLocked {
		completed = 1
		if earlyErr != nil {
			t.Errorf("先に完了した並行処理のエラー = %v", earlyErr)
		}
	}
	for ; completed < 2; completed++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("並行処理のエラー = %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("並行処理が完了しませんでした: %v", ctx.Err())
		}
	}
	if completedWhileLocked {
		t.Error("ユーザーの行のロック中にトークンの送信が完了しました")
	}
	if len(sender.tokens) != 2 {
		t.Fatalf("送信したトークンの件数 = %d、期待値 = 2", len(sender.tokens))
	}
	if sender.tokens[0] == sender.tokens[1] {
		t.Error("並行処理が同じトークンを送信しました")
	}
	tokens := passwordResetTokens(t, userID)
	if len(tokens) != 1 {
		t.Fatalf("保存したトークンの件数 = %d、期待値 = 1", len(tokens))
	}
	matching := 0
	for _, token := range sender.tokens {
		if _, ok := tokens[auth.HashToken(token)]; ok {
			matching++
		}
	}
	if matching != 1 {
		t.Errorf("送信したトークンのうち保存された件数 = %d、期待値 = 1", matching)
	}
}

// newSendPasswordResetUsecase はテスト用のデータベースに直接書き込む SendPasswordResetUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newSendPasswordResetUsecase(sender usecase.PasswordResetSender) *usecase.SendPasswordResetUsecase {
	db := testutil.GetTestDB()

	return usecase.NewSendPasswordResetUsecase(
		db,
		repository.NewUserRepository(db),
		repository.NewPasswordResetTokenRepository(db),
		sender,
	)
}

// passwordResetTokens はユーザーのトークンのダイジェストと有効期限を返す。
func passwordResetTokens(t *testing.T, userID model.UserID) map[string]time.Time {
	t.Helper()

	rows, err := testutil.GetTestDB().QueryContext(context.Background(),
		"SELECT token_digest, expires_at FROM password_reset_tokens WHERE user_id = $1", uuid.UUID(userID))
	if err != nil {
		t.Fatalf("トークンの取得のエラー = %v", err)
	}
	defer func() { _ = rows.Close() }()

	tokens := map[string]time.Time{}
	for rows.Next() {
		var digest string
		var expiresAt time.Time
		if err := rows.Scan(&digest, &expiresAt); err != nil {
			t.Fatalf("トークンの読み取りのエラー = %v", err)
		}
		tokens[digest] = expiresAt
	}

	return tokens
}

// TestSendPasswordResetUsecase_Execute は、ユーザーのアドレスへトークンを送り、そのダイジェストだけを1時間の期限で保存することと、
// 申請し直すと以前のトークンを置き換えることを検証する。
func TestSendPasswordResetUsecase_Execute(t *testing.T) {
	t.Parallel()

	sender := &fakePasswordResetSender{}
	uc := newSendPasswordResetUsecase(sender)
	email := testutil.UniqueEmail("send-password-reset")
	userID := testutil.NewUserBuilder(t, testutil.GetTestDB()).WithEmail(email).Build()

	for i := range 2 {
		before := time.Now()
		if err := uc.Execute(context.Background(), usecase.SendPasswordResetInput{UserID: userID, Locale: model.LocaleEn}); err != nil {
			t.Fatalf("%d回目: Execute()のエラー = %v", i+1, err)
		}

		if len(sender.tokens) != i+1 || sender.to[i] != email || sender.locale[i] != model.LocaleEn {
			t.Fatalf("%d回目: 送ったメール = (%v, %v)、%s へ英語で送ることを期待", i+1, sender.to, sender.locale, email)
		}
		tokens := passwordResetTokens(t, userID)
		expiresAt, ok := tokens[auth.HashToken(sender.tokens[i])]
		if len(tokens) != 1 || !ok {
			t.Fatalf("%d回目: 保存したトークン = %v、送ったトークンのダイジェスト1件だけを期待", i+1, tokens)
		}
		if expiresAt.Before(before.Add(model.PasswordResetTokenLifetime)) || expiresAt.After(time.Now().Add(model.PasswordResetTokenLifetime)) {
			t.Errorf("%d回目: 有効期限 = %v、送った時刻の1時間後を期待", i+1, expiresAt)
		}
	}
	if sender.tokens[0] == sender.tokens[1] {
		t.Error("申請し直しても同じトークンを送った")
	}
}

// TestSendPasswordResetUsecase_Execute_WithdrawnUser は、退会したユーザーにはトークンを作らず、メールも送らないことを検証する。
func TestSendPasswordResetUsecase_Execute_WithdrawnUser(t *testing.T) {
	t.Parallel()

	sender := &fakePasswordResetSender{}
	uc := newSendPasswordResetUsecase(sender)
	userID := testutil.NewUserBuilder(t, testutil.GetTestDB()).WithDeletedAt(time.Now()).Build()

	if err := uc.Execute(context.Background(), usecase.SendPasswordResetInput{UserID: userID, Locale: model.LocaleJa}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(sender.to) != 0 {
		t.Errorf("送ったメール = %v、送らないことを期待", sender.to)
	}
	if tokens := passwordResetTokens(t, userID); len(tokens) != 0 {
		t.Errorf("保存したトークン = %v、作らないことを期待", tokens)
	}
}

// TestSendPasswordResetUsecase_Execute_SendError は、送信の失敗をジョブの再試行に任せるため、エラーとして返すことを検証する。
func TestSendPasswordResetUsecase_Execute_SendError(t *testing.T) {
	t.Parallel()

	sendErr := errors.New("Resendの障害")
	uc := newSendPasswordResetUsecase(&fakePasswordResetSender{err: sendErr})
	userID := testutil.NewUserBuilder(t, testutil.GetTestDB()).Build()

	if err := uc.Execute(context.Background(), usecase.SendPasswordResetInput{UserID: userID, Locale: model.LocaleJa}); !errors.Is(err, sendErr) {
		t.Errorf("Execute()のエラー = %v、送信のエラーを期待", err)
	}
}
