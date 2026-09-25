package testutil_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestUserPasswordBuilder_Build は、ビルダーが投入した資格情報が既定のパスワードと
// 照合できることを検証する。以降のテストがログインの前提を用意する経路になるため。
func TestUserPasswordBuilder_Build(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserPasswordRepository(db).WithTx(tx)

	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).Build()

	userPassword, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}

	if userPassword == nil {
		t.Fatal("パスワード資格情報 = nil、非nilを期待")
	}

	if err := auth.CheckPassword(userPassword.PasswordDigest, testutil.DefaultBuilderPassword); err != nil {
		t.Errorf("ビルダーが投入したダイジェストと既定のパスワードの照合に失敗: %v", err)
	}
}
