package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/worker"
)

// TestDeleteExpiredEmailConfirmationsWorker_Work は、ジョブの処理で保持期間を過ぎた確認が消えることを検証する。
func TestDeleteExpiredEmailConfirmationsWorker_Work(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	uc := usecase.NewDeleteExpiredEmailConfirmationsUsecase(repository.NewEmailConfirmationRepository(db).WithTx(tx))
	w := worker.NewDeleteExpiredEmailConfirmationsWorker(uc)

	expiredID := testutil.NewEmailConfirmationBuilder(t, tx).WithExpiresAt(time.Now().Add(-48 * time.Hour)).Build()

	job := &river.Job[dispatcher.DeleteExpiredEmailConfirmationsArgs]{Args: dispatcher.DeleteExpiredEmailConfirmationsArgs{}}
	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM email_confirmations WHERE id = $1)", expiredID.String()).Scan(&exists); err != nil {
		t.Fatalf("確認の存在確認のエラー = %v", err)
	}
	if exists {
		t.Error("保持期間を過ぎた確認が残っている")
	}
}
