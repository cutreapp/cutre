package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestUserPasswordRepository_Create は、保存したダイジェストがそのまま取り出せて、
// 元の平文と照合できることを検証する。
func TestUserPasswordRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserPasswordRepository(db).WithTx(tx)

	userID := testutil.NewUserBuilder(t, tx).Build()

	const password = "password123"
	digest, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword()のエラー = %v", err)
	}

	created, err := repo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         userID,
		PasswordDigest: digest,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if created.UserID != userID {
		t.Errorf("UserID = %s、期待値 = %s", created.UserID, userID)
	}

	found, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}

	if found == nil {
		t.Fatal("パスワード資格情報 = nil、非nilを期待")
	}

	if found.ID != created.ID {
		t.Errorf("ID = %s、期待値 = %s", found.ID, created.ID)
	}

	if err := auth.CheckPassword(found.PasswordDigest, password); err != nil {
		t.Errorf("保存したダイジェストと元のパスワードの照合に失敗: %v", err)
	}
}

// TestUserPasswordRepository_FindByUserID_NotFound は、資格情報を持たないユーザーと
// 存在しないユーザーのどちらでも (nil, nil) を返すことを検証する。
func TestUserPasswordRepository_FindByUserID_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserPasswordRepository(db).WithTx(tx)

	tests := []struct {
		name   string
		userID model.UserID
	}{
		{name: "パスワードを持たないユーザー", userID: testutil.NewUserBuilder(t, tx).Build()},
		{name: "存在しないユーザー", userID: model.UserID(uuid.New())},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userPassword, err := repo.FindByUserID(ctx, tt.userID)
			if err != nil {
				t.Fatalf("FindByUserID()のエラー = %v", err)
			}

			if userPassword != nil {
				t.Errorf("パスワード資格情報 = %v、期待値 = nil", userPassword)
			}
		})
	}
}

// TestUserPasswordRepository_UpdatePasswordDigest は、対象のユーザーのダイジェストだけを置き換え、
// パスワードを持たないユーザーではfalseを返すことを検証する。
func TestUserPasswordRepository_UpdatePasswordDigest(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserPasswordRepository(db).WithTx(tx)

	targetID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(targetID).Build()
	otherID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(otherID).Build()

	const newPassword = "new-password123"
	digest, err := auth.HashPassword(newPassword)
	if err != nil {
		t.Fatalf("HashPassword()のエラー = %v", err)
	}

	updated, err := repo.UpdatePasswordDigest(ctx, targetID, digest)
	if err != nil || !updated {
		t.Fatalf("UpdatePasswordDigest() = (%t, %v)、期待値 = (true, nil)", updated, err)
	}

	target, err := repo.FindByUserID(ctx, targetID)
	if err != nil || auth.CheckPassword(target.PasswordDigest, newPassword) != nil {
		t.Errorf("対象のユーザーのパスワード = (%+v, %v)、新しいパスワードとの一致を期待", target, err)
	}
	other, err := repo.FindByUserID(ctx, otherID)
	if err != nil || auth.CheckPassword(other.PasswordDigest, testutil.DefaultBuilderPassword) != nil {
		t.Errorf("ほかのユーザーのパスワード = (%+v, %v)、元のパスワードのままを期待", other, err)
	}

	updated, err = repo.UpdatePasswordDigest(ctx, testutil.NewUserBuilder(t, tx).Build(), digest)
	if err != nil || updated {
		t.Errorf("パスワードを持たないユーザーのUpdatePasswordDigest() = (%t, %v)、期待値 = (false, nil)", updated, err)
	}
}

// TestUserPasswordRepository_DeleteByUserID は、指定したユーザーのパスワードだけを消し、無いときも失敗しないことを検証する。
func TestUserPasswordRepository_DeleteByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserPasswordRepository(db).WithTx(tx)

	targetID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(targetID).Build()
	otherID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(otherID).Build()

	for range 2 {
		if err := repo.DeleteByUserID(ctx, targetID); err != nil {
			t.Fatalf("DeleteByUserID()のエラー = %v", err)
		}
	}

	if target, err := repo.FindByUserID(ctx, targetID); err != nil || target != nil {
		t.Errorf("対象のユーザーのパスワード = (%+v, %v)、(nil, nil) を期待", target, err)
	}
	if other, err := repo.FindByUserID(ctx, otherID); err != nil || other == nil {
		t.Errorf("ほかのユーザーのパスワード = (%+v, %v)、残っていることを期待", other, err)
	}
}
