package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestDeleteSessionUsecase_Execute は、トークンが指すセッションだけを削除し、
// 同じユーザーの他のセッションは残すことを検証する。
func TestDeleteSessionUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)
	uc := usecase.NewDeleteSessionUsecase(repo)

	userID := testutil.NewUserBuilder(t, tx).Build()
	target := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	target.Build()
	other := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	other.Build()

	if err := uc.Execute(ctx, usecase.DeleteSessionInput{Token: target.Token()}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if found, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(target.Token())); err != nil || found != nil {
		t.Errorf("削除したセッション = (%v, %v)、(nil, nil)を期待", found, err)
	}
	if found, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(other.Token())); err != nil || found == nil {
		t.Errorf("他の端末のセッション = (%v, %v)、残ることを期待", found, err)
	}
}

// TestDeleteSessionUsecase_Execute_NoSession は、ログインしていない (トークンが空・未知の) ログアウトを
// エラーにしないことを検証する。
func TestDeleteSessionUsecase_Execute_NoSession(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewDeleteSessionUsecase(repository.NewUserSessionRepository(db).WithTx(tx))

	for _, token := range []string{"", "unknown-token"} {
		if err := uc.Execute(context.Background(), usecase.DeleteSessionInput{Token: token}); err != nil {
			t.Errorf("Execute(%q)のエラー = %v、nilを期待", token, err)
		}
	}
}

// TestDeleteSessionUsecase_Execute_Error は、Repositoryの削除エラーを呼び出し元へ返し、
// セッションを残すことを検証する。
func TestDeleteSessionUsecase_Execute_Error(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserSessionRepository(db).WithTx(tx)
	uc := usecase.NewDeleteSessionUsecase(repo)

	userID := testutil.NewUserBuilder(t, tx).Build()
	userSession := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	userSession.Build()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := uc.Execute(ctx, usecase.DeleteSessionInput{Token: userSession.Token()}); !errors.Is(err, context.Canceled) {
		t.Errorf("Execute()のエラー = %v、context.Canceledを期待", err)
	}

	found, err := repo.FindLiveWithUserByTokenDigest(context.Background(), auth.HashToken(userSession.Token()))
	if err != nil || found == nil {
		t.Errorf("削除失敗後のセッション = (%v, %v)、残ることを期待", found, err)
	}
}
