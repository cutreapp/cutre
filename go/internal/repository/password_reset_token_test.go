package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// countPasswordResetTokens はユーザーのパスワードリセットのトークンの件数を返す。
func countPasswordResetTokens(t *testing.T, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, userID model.UserID) int {
	t.Helper()

	var count int
	if err := q.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = $1", uuid.UUID(userID),
	).Scan(&count); err != nil {
		t.Fatalf("トークンの件数の取得のエラー = %v", err)
	}

	return count
}

// TestPasswordResetTokenRepository_Create は、トークンを入力のユーザー・ダイジェスト・有効期限で作って返すことを検証する。
func TestPasswordResetTokenRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewPasswordResetTokenRepository(db).WithTx(tx)

	userID := testutil.NewUserBuilder(t, tx).Build()
	expiresAt := model.PasswordResetTokenExpiresAt(time.Now()).Truncate(time.Microsecond)

	token, err := repo.Create(context.Background(), repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "digest-create",
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if token.UserID != userID || token.TokenDigest != "digest-create" || !token.ExpiresAt.Equal(expiresAt) {
		t.Errorf("トークン = %+v、入力のユーザー・ダイジェスト・有効期限を期待", token)
	}
}

// TestPasswordResetTokenRepository_Create_OnePerUser は、同じユーザーに2件目のトークンを保存できないことを検証する。
func TestPasswordResetTokenRepository_Create_OnePerUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewPasswordResetTokenRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	ctx := context.Background()

	for i := range 2 {
		_, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
			UserID:      userID,
			TokenDigest: uuid.NewString(),
			ExpiresAt:   model.PasswordResetTokenExpiresAt(time.Now()),
		})
		if i == 0 && err != nil {
			t.Fatalf("1件目のCreate()のエラー = %v", err)
		}
		if i == 1 && err == nil {
			t.Error("同じユーザーに2件目のトークンを保存できました")
		}
	}
}

// TestPasswordResetTokenRepository_DeleteByUserID は、指定したユーザーのトークンだけを消すことを検証する。
func TestPasswordResetTokenRepository_DeleteByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewPasswordResetTokenRepository(db).WithTx(tx)
	ctx := context.Background()

	target := testutil.NewUserBuilder(t, tx).Build()
	other := testutil.NewUserBuilder(t, tx).Build()
	for i, userID := range []model.UserID{target, other} {
		if _, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
			UserID:      userID,
			TokenDigest: uuid.NewString(),
			ExpiresAt:   model.PasswordResetTokenExpiresAt(time.Now()),
		}); err != nil {
			t.Fatalf("%d件目のCreate()のエラー = %v", i+1, err)
		}
	}

	if err := repo.DeleteByUserID(ctx, target); err != nil {
		t.Fatalf("DeleteByUserID()のエラー = %v", err)
	}

	if got := countPasswordResetTokens(t, tx, target); got != 0 {
		t.Errorf("対象のユーザーのトークンの件数 = %d、期待値 = 0", got)
	}
	if got := countPasswordResetTokens(t, tx, other); got != 1 {
		t.Errorf("ほかのユーザーのトークンの件数 = %d、期待値 = 1", got)
	}
}

// TestPasswordResetTokenRepository_FindLive は、ダイジェストとIDのどちらでも期限内のトークンだけを返し、
// 期限切れと無いトークンにはnilを返すことを検証する。
func TestPasswordResetTokenRepository_FindLive(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewPasswordResetTokenRepository(db).WithTx(tx)
	ctx := context.Background()

	live := testutil.NewPasswordResetTokenBuilder(t, tx).WithUserID(testutil.NewUserBuilder(t, tx).Build())
	liveID := live.Build()
	expired := testutil.NewPasswordResetTokenBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build()).
		WithExpiresAt(time.Now().Add(-time.Minute))
	expiredID := expired.Build()

	tests := []struct {
		name   string
		find   func() (*model.PasswordResetToken, error)
		wantID *model.PasswordResetTokenID
	}{
		{name: "期限内のトークンをダイジェストで引く", find: func() (*model.PasswordResetToken, error) {
			return repo.FindLiveByTokenDigest(ctx, auth.HashToken(live.Token()))
		}, wantID: &liveID},
		{name: "期限内のトークンをIDで引く", find: func() (*model.PasswordResetToken, error) {
			return repo.FindLiveByID(ctx, liveID)
		}, wantID: &liveID},
		{name: "期限切れのトークンをダイジェストで引く", find: func() (*model.PasswordResetToken, error) {
			return repo.FindLiveByTokenDigest(ctx, auth.HashToken(expired.Token()))
		}},
		{name: "期限切れのトークンをIDで引く", find: func() (*model.PasswordResetToken, error) {
			return repo.FindLiveByID(ctx, expiredID)
		}},
		{name: "無いトークン", find: func() (*model.PasswordResetToken, error) {
			return repo.FindLiveByTokenDigest(ctx, auth.HashToken("no-such-token"))
		}},
	}

	for _, tt := range tests {
		got, err := tt.find()
		if err != nil {
			t.Errorf("%s: エラー = %v", tt.name, err)
			continue
		}
		switch {
		case tt.wantID == nil && got != nil:
			t.Errorf("%s: トークン = %+v、nilを期待", tt.name, got)
		case tt.wantID != nil && (got == nil || got.ID != *tt.wantID):
			t.Errorf("%s: トークン = %+v、ID %v を期待", tt.name, got, *tt.wantID)
		}
	}
}

// TestPasswordResetTokenRepository_DeleteLive は、期限内のトークンを1度だけ消せ、期限切れのトークンは消さないことを検証する。
func TestPasswordResetTokenRepository_DeleteLive(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewPasswordResetTokenRepository(db).WithTx(tx)
	ctx := context.Background()

	liveUserID := testutil.NewUserBuilder(t, tx).Build()
	liveID := testutil.NewPasswordResetTokenBuilder(t, tx).WithUserID(liveUserID).Build()
	expiredUserID := testutil.NewUserBuilder(t, tx).Build()
	expiredID := testutil.NewPasswordResetTokenBuilder(t, tx).WithUserID(expiredUserID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()

	for i, want := range []bool{true, false} {
		deleted, err := repo.DeleteLive(ctx, liveID)
		if err != nil || deleted != want {
			t.Errorf("%d回目のDeleteLive() = (%t, %v)、期待値 = (%t, nil)", i+1, deleted, err, want)
		}
	}
	if got := countPasswordResetTokens(t, tx, liveUserID); got != 0 {
		t.Errorf("使ったトークンの件数 = %d、期待値 = 0", got)
	}

	if deleted, err := repo.DeleteLive(ctx, expiredID); err != nil || deleted {
		t.Errorf("期限切れのトークンのDeleteLive() = (%t, %v)、期待値 = (false, nil)", deleted, err)
	}
	if got := countPasswordResetTokens(t, tx, expiredUserID); got != 1 {
		t.Errorf("期限切れのトークンの件数 = %d、期待値 = 1", got)
	}
}
