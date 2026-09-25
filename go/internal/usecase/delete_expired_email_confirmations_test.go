package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestDeleteExpiredEmailConfirmationsUsecase_Execute は、有効期限から保持期間 (1日) を過ぎた確認だけを確認済みかどうかによらず消し、
// 期限切れでも保持期間の内側にある確認と、期限内の確認は残すことを検証する。
func TestDeleteExpiredEmailConfirmationsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	uc := usecase.NewDeleteExpiredEmailConfirmationsUsecase(repository.NewEmailConfirmationRepository(db).WithTx(tx))

	now := time.Now()
	pastRetention := now.Add(-25 * time.Hour)
	unconfirmedID := testutil.NewEmailConfirmationBuilder(t, tx).WithExpiresAt(pastRetention).Build()
	confirmedID := testutil.NewEmailConfirmationBuilder(t, tx).
		WithExpiresAt(pastRetention).
		WithConfirmedAt(pastRetention.Add(-time.Minute)).
		Build()
	// 確認済みのCookieが残りうる、有効期限の直後の確認は消さない。
	retainedID := testutil.NewEmailConfirmationBuilder(t, tx).
		WithExpiresAt(now.Add(-30 * time.Minute)).
		WithConfirmedAt(now.Add(-35 * time.Minute)).
		Build()
	liveID := testutil.NewEmailConfirmationBuilder(t, tx).Build()

	if err := uc.Execute(ctx); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	tests := []struct {
		name string
		id   model.EmailConfirmationID
		want bool
	}{
		{name: "保持期間を過ぎた未確認の確認", id: unconfirmedID, want: false},
		{name: "保持期間を過ぎた確認済みの確認", id: confirmedID, want: false},
		{name: "保持期間の内側の期限切れの確認", id: retainedID, want: true},
		{name: "期限内の確認", id: liveID, want: true},
	}
	for _, tt := range tests {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM email_confirmations WHERE id = $1)", uuid.UUID(tt.id)).Scan(&exists); err != nil {
			t.Fatalf("確認の存在確認のエラー = %v", err)
		}
		if exists != tt.want {
			t.Errorf("%sが残っているか = %t、期待値 = %t", tt.name, exists, tt.want)
		}
	}
}

// TestDeleteExpiredEmailConfirmationsUsecase_Execute_Error は、Repositoryの削除エラーを呼び出し元へ返すことを検証する。
// 定期ジョブはこのエラーを見てRiverに再試行させる。
func TestDeleteExpiredEmailConfirmationsUsecase_Execute_Error(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewDeleteExpiredEmailConfirmationsUsecase(repository.NewEmailConfirmationRepository(db).WithTx(tx))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := uc.Execute(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Execute()のエラー = %v、context.Canceledを期待", err)
	}
}
