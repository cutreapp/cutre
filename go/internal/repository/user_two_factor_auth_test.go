package repository_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// createPendingTwoFactorAuth はユーザーの登録の途中の二要素認証の設定を作る。
func createPendingTwoFactorAuth(t *testing.T, repo *repository.UserTwoFactorAuthRepository, userID model.UserID, secretCiphertext []byte) *model.UserTwoFactorAuth {
	t.Helper()

	setting, err := repo.UpsertPending(context.Background(), userID, secretCiphertext)
	if err != nil || setting == nil {
		t.Fatalf("UpsertPending() = (%+v, %v)、設定とnilを期待", setting, err)
	}

	return setting
}

// TestUserTwoFactorAuthRepository_UpsertPending は、登録の途中の設定を作り、開き直したときは秘密鍵を差し替え、
// 有効にした設定は書き換えないことを検証する。
func TestUserTwoFactorAuthRepository_UpsertPending(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserTwoFactorAuthRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	ctx := context.Background()

	created := createPendingTwoFactorAuth(t, repo, userID, []byte("first"))
	if created.UserID != userID || !bytes.Equal(created.SecretCiphertext, []byte("first")) || created.IsEnabled() || created.LastUsedStep != 0 {
		t.Errorf("作った設定 = %+v、入力のユーザーと秘密鍵の登録の途中の設定を期待", created)
	}

	replaced := createPendingTwoFactorAuth(t, repo, userID, []byte("second"))
	if replaced.ID != created.ID || !bytes.Equal(replaced.SecretCiphertext, []byte("second")) {
		t.Errorf("差し替えた設定 = %+v、同じ行で秘密鍵が second になることを期待", replaced)
	}

	if ok, err := repo.Enable(ctx, userID, replaced.SecretCiphertext, 100); err != nil || !ok {
		t.Fatalf("Enable() = (%t, %v)、期待値 = (true, nil)", ok, err)
	}
	if got, err := repo.UpsertPending(ctx, userID, []byte("third")); err != nil || got != nil {
		t.Errorf("有効にした後のUpsertPending() = (%+v, %v)、期待値 = (nil, nil)", got, err)
	}

	found, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if !bytes.Equal(found.SecretCiphertext, []byte("second")) {
		t.Errorf("有効にした設定の秘密鍵 = %q、期待値 = %q", found.SecretCiphertext, "second")
	}
}

// TestUserTwoFactorAuthRepository_FindByUserID は、設定の無いユーザーにnilを返すことを検証する。
func TestUserTwoFactorAuthRepository_FindByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserTwoFactorAuthRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()

	got, err := repo.FindByUserID(context.Background(), userID)
	if err != nil || got != nil {
		t.Errorf("FindByUserID() = (%+v, %v)、期待値 = (nil, nil)", got, err)
	}
}

// TestUserTwoFactorAuthRepository_Enable は、登録の途中の設定だけを有効にして照合したステップを記録し、
// 設定が無い・既に有効・秘密鍵が違うときは更新しないことを検証する。
func TestUserTwoFactorAuthRepository_Enable(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserTwoFactorAuthRepository(db).WithTx(tx)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, tx).Build()
	setting := createPendingTwoFactorAuth(t, repo, userID, []byte("secret"))

	if ok, err := repo.Enable(ctx, userID, []byte("other"), 100); err != nil || ok {
		t.Errorf("秘密鍵が違うEnable() = (%t, %v)、期待値 = (false, nil)", ok, err)
	}

	for i, want := range []bool{true, false} {
		ok, err := repo.Enable(ctx, userID, setting.SecretCiphertext, 100)
		if err != nil || ok != want {
			t.Errorf("%d回目のEnable() = (%t, %v)、期待値 = (%t, nil)", i+1, ok, err, want)
		}
	}

	found, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if !found.IsEnabled() || found.LastUsedStep != 100 {
		t.Errorf("有効にした設定 = %+v、有効でステップ100を記録していることを期待", found)
	}

	noAuthUserID := testutil.NewUserBuilder(t, tx).Build()
	if ok, err := repo.Enable(ctx, noAuthUserID, setting.SecretCiphertext, 100); err != nil || ok {
		t.Errorf("設定の無いユーザーのEnable() = (%t, %v)、期待値 = (false, nil)", ok, err)
	}
}

// TestUserTwoFactorAuthRepository_Enable_SecretReplaced は、コード照合後に秘密鍵が差し替わったときに
// 古い秘密鍵での有効化を拒み、現在の秘密鍵は有効化できることを検証する。
func TestUserTwoFactorAuthRepository_Enable_SecretReplaced(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserTwoFactorAuthRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	ctx := context.Background()

	previous := createPendingTwoFactorAuth(t, repo, userID, []byte("previous"))
	current := createPendingTwoFactorAuth(t, repo, userID, []byte("current"))

	if ok, err := repo.Enable(ctx, userID, previous.SecretCiphertext, 100); err != nil || ok {
		t.Errorf("差し替え前の秘密鍵でのEnable() = (%t, %v)、期待値 = (false, nil)", ok, err)
	}

	found, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if found.IsEnabled() || found.LastUsedStep != 0 {
		t.Errorf("古い秘密鍵での照合後の設定 = %+v、登録の途中でステップ未使用を期待", found)
	}

	if ok, err := repo.Enable(ctx, userID, current.SecretCiphertext, 100); err != nil || !ok {
		t.Errorf("現在の秘密鍵でのEnable() = (%t, %v)、期待値 = (true, nil)", ok, err)
	}
}

// TestUserTwoFactorAuthRepository_UseStep は、記録済みより新しいステップだけを受け付け、
// 同じ・古いステップと登録の途中の設定は拒むことを検証する。
func TestUserTwoFactorAuthRepository_UseStep(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserTwoFactorAuthRepository(db).WithTx(tx)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, tx).Build()
	setting := createPendingTwoFactorAuth(t, repo, userID, []byte("secret"))

	if ok, err := repo.UseStep(ctx, userID, 100); err != nil || ok {
		t.Errorf("登録の途中のUseStep() = (%t, %v)、期待値 = (false, nil)", ok, err)
	}

	if ok, err := repo.Enable(ctx, userID, setting.SecretCiphertext, 100); err != nil || !ok {
		t.Fatalf("Enable() = (%t, %v)、期待値 = (true, nil)", ok, err)
	}

	tests := []struct {
		name string
		step int64
		want bool
	}{
		{name: "有効にしたときと同じステップ", step: 100, want: false},
		{name: "古いステップ", step: 99, want: false},
		{name: "新しいステップ", step: 101, want: true},
		{name: "使ったばかりのステップ", step: 101, want: false},
	}

	for _, tt := range tests {
		ok, err := repo.UseStep(ctx, userID, tt.step)
		if err != nil || ok != tt.want {
			t.Errorf("%s: UseStep() = (%t, %v)、期待値 = (%t, nil)", tt.name, ok, err, tt.want)
		}
	}
}

// TestUserTwoFactorAuthRepository_DeleteByUserID は、指定したユーザーの設定だけを消すことを検証する。
func TestUserTwoFactorAuthRepository_DeleteByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewUserTwoFactorAuthRepository(db).WithTx(tx)
	ctx := context.Background()

	target := testutil.NewUserBuilder(t, tx).Build()
	other := testutil.NewUserBuilder(t, tx).Build()
	createPendingTwoFactorAuth(t, repo, target, []byte("target"))
	createPendingTwoFactorAuth(t, repo, other, []byte("other"))

	if err := repo.DeleteByUserID(ctx, target); err != nil {
		t.Fatalf("DeleteByUserID()のエラー = %v", err)
	}

	if got, err := repo.FindByUserID(ctx, target); err != nil || got != nil {
		t.Errorf("対象のユーザーのFindByUserID() = (%+v, %v)、期待値 = (nil, nil)", got, err)
	}
	if got, err := repo.FindByUserID(ctx, other); err != nil || got == nil {
		t.Errorf("ほかのユーザーのFindByUserID() = (%+v, %v)、設定が残ることを期待", got, err)
	}
}
