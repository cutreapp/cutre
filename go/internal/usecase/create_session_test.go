package usecase_test

import (
	"context"
	"os"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// TestCreateSessionUsecase_Execute は、返したトークンでセッションとそのユーザーを引けること、
// そしてデータベースには平文ではなくダイジェストが保存されることを検証する。
func TestCreateSessionUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)
	uc := usecase.NewCreateSessionUsecase(repo)

	userID := testutil.NewUserBuilder(t, tx).Build()

	output, err := uc.Execute(ctx, usecase.CreateSessionInput{
		UserID:    userID,
		IPAddress: "203.0.113.10",
		UserAgent: "test-agent",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Token == "" {
		t.Fatal("Token = 空文字列、非空を期待")
	}

	userSession, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(output.Token))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
	}
	if userSession == nil {
		t.Fatal("セッション = nil、非nilを期待")
	}
	if userSession.User.ID != userID {
		t.Errorf("UserID = %s、期待値 = %s", userSession.User.ID, userID)
	}
	if userSession.TokenDigest == output.Token {
		t.Error("TokenDigest が平文のトークンと同じ値になっている")
	}
	if userSession.IPAddress != "203.0.113.10" || userSession.UserAgent != "test-agent" {
		t.Errorf("接続元 = (%q, %q)、期待値 = (%q, %q)", userSession.IPAddress, userSession.UserAgent, "203.0.113.10", "test-agent")
	}
}
