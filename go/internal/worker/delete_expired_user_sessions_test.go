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

// TestDeleteExpiredUserSessionsWorker_Work は、ジョブの処理で期限切れのセッションが消えることを検証する。
func TestDeleteExpiredUserSessionsWorker_Work(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	uc := usecase.NewDeleteExpiredUserSessionsUsecase(repository.NewUserSessionRepository(db).WithTx(tx))
	w := worker.NewDeleteExpiredUserSessionsWorker(uc)

	userID := testutil.NewUserBuilder(t, tx).Build()
	expiredID := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()

	job := &river.Job[dispatcher.DeleteExpiredUserSessionsArgs]{Args: dispatcher.DeleteExpiredUserSessionsArgs{}}
	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM user_sessions WHERE id = $1)", expiredID.String()).Scan(&exists); err != nil {
		t.Fatalf("セッションの存在確認のエラー = %v", err)
	}
	if exists {
		t.Error("期限切れのセッションが残っている")
	}
}
