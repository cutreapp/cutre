package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestDeleteExpiredUserSessionsUsecase_Execute は、期限切れのセッションを消し、期限内のセッションを残すことを検証する。
func TestDeleteExpiredUserSessionsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)
	uc := usecase.NewDeleteExpiredUserSessionsUsecase(repo)

	userID := testutil.NewUserBuilder(t, tx).Build()
	expiredID := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()
	live := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID).WithExpiresAt(time.Now().Add(time.Hour))
	live.Build()

	if err := uc.Execute(ctx); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	var remaining int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_sessions WHERE id = $1", expiredID.String()).Scan(&remaining); err != nil {
		t.Fatalf("セッションの件数の取得のエラー = %v", err)
	}
	if remaining != 0 {
		t.Errorf("期限切れのセッションの件数 = %d、期待値 = 0", remaining)
	}

	if found, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(live.Token())); err != nil || found == nil {
		t.Errorf("期限内のセッション = (%v, %v)、残ることを期待", found, err)
	}
}

// TestDeleteExpiredUserSessionsUsecase_Execute_Error は、Repositoryの削除エラーを呼び出し元へ返すことを検証する。
// 定期ジョブはこのエラーを見てRiverに再試行させる。
func TestDeleteExpiredUserSessionsUsecase_Execute_Error(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewDeleteExpiredUserSessionsUsecase(repository.NewUserSessionRepository(db).WithTx(tx))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := uc.Execute(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Execute()のエラー = %v、context.Canceledを期待", err)
	}
}
