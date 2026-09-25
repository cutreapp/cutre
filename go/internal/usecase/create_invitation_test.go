package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestCreateInvitationUsecase_Execute は、招待者の無い招待を7日の期限で発行し、招待ごとに別のトークンを振ることを検証する。
func TestCreateInvitationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	uc := usecase.NewCreateInvitationUsecase(repository.NewInvitationRepository(db).WithTx(tx))

	before := time.Now()
	first, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	after := time.Now()

	invitation := first.Invitation
	if invitation.InviterUserID != nil {
		t.Errorf("InviterUserID = %v、期待値 = nil", invitation.InviterUserID)
	}
	if invitation.Token == "" {
		t.Error("Token = 空文字列、非空を期待")
	}
	if invitation.ExpiresAt.Before(before.Add(model.InvitationLifetime).Truncate(time.Microsecond)) ||
		invitation.ExpiresAt.After(after.Add(model.InvitationLifetime)) {
		t.Errorf("ExpiresAt = %v、期待値 = 発行から%sの後", invitation.ExpiresAt, model.InvitationLifetime)
	}

	second, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if second.Invitation.Token == invitation.Token {
		t.Error("2つの招待のトークンが同じ、別の値を期待")
	}
}
