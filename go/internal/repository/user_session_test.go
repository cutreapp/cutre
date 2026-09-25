package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestUserSessionRepository_Create は、作成したセッションがダイジェストで引けて、
// 持ち主のユーザーが一緒に返ることを検証する。
func TestUserSessionRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)

	userID := testutil.NewUserBuilder(t, tx).Build()
	const token = "plain-token"
	// PostgreSQLのtimestamptzはマイクロ秒までしか持たないため、比較する値も同じ精度に落とす。
	expiresAt := model.UserSessionExpiresAt(time.Now().Truncate(time.Microsecond))

	created, err := repo.Create(ctx, repository.CreateUserSessionInput{
		UserID:      userID,
		TokenDigest: auth.HashToken(token),
		ExpiresAt:   expiresAt,
		IPAddress:   "192.0.2.1",
		UserAgent:   "cutre-test",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if created.UserID != userID {
		t.Errorf("UserID = %s、期待値 = %s", created.UserID, userID)
	}
	if created.TokenDigest == token {
		t.Error("平文のトークンがそのまま保存されている")
	}
	if !created.ExpiresAt.Equal(expiresAt) {
		t.Errorf("ExpiresAt = %v、期待値 = %v", created.ExpiresAt, expiresAt)
	}

	found, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(token))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
	}
	if found == nil {
		t.Fatal("セッション = nil、非nilを期待")
	}
	if found.ID != created.ID {
		t.Errorf("ID = %s、期待値 = %s", found.ID, created.ID)
	}
	if found.IPAddress != "192.0.2.1" {
		t.Errorf("IPAddress = %q、期待値 = %q", found.IPAddress, "192.0.2.1")
	}
	if found.User == nil {
		t.Fatal("User = nil、非nilを期待")
	}
	if found.User.ID != userID {
		t.Errorf("User.ID = %s、期待値 = %s", found.User.ID, userID)
	}
}

// TestUserSessionRepository_FindLiveWithUserByTokenDigest_NotFound は、
// ログインとして扱ってはいけないセッションが引けないことを検証する。
func TestUserSessionRepository_FindLiveWithUserByTokenDigest_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)

	expiredBuilder := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build()).
		WithExpiresAt(time.Now().Add(-time.Minute))
	expiredBuilder.Build()

	withdrawnBuilder := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build())
	withdrawnBuilder.Build()

	tests := []struct {
		name  string
		token string
	}{
		{name: "未知のトークン", token: "unknown-token"},
		{name: "期限切れのセッション", token: expiredBuilder.Token()},
		{name: "退会したユーザーのセッション", token: withdrawnBuilder.Token()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(tt.token))
			if err != nil {
				t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
			}

			if session != nil {
				t.Errorf("セッション = %v、期待値 = nil", session)
			}
		})
	}
}

// TestUserSessionRepository_Extend は、有効期限と最終利用時刻が進むことを検証する。
func TestUserSessionRepository_Extend(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)

	builder := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build()).
		WithLastSeenAt(time.Now().Add(-48 * time.Hour))
	sessionID := builder.Build()

	now := time.Now().Truncate(time.Microsecond)
	expiresAt := model.UserSessionExpiresAt(now)

	if err := repo.Extend(ctx, sessionID, expiresAt, now); err != nil {
		t.Fatalf("Extend()のエラー = %v", err)
	}

	extended, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(builder.Token()))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
	}
	if extended == nil {
		t.Fatal("セッション = nil、非nilを期待")
	}
	if !extended.ExpiresAt.Equal(expiresAt) {
		t.Errorf("ExpiresAt = %v、期待値 = %v", extended.ExpiresAt, expiresAt)
	}
	if !extended.LastSeenAt.Equal(now) {
		t.Errorf("LastSeenAt = %v、期待値 = %v", extended.LastSeenAt, now)
	}
}

// TestUserSessionRepository_DeleteByTokenDigest は、ログアウトしたセッションが引けなくなり、
// 既に無いセッションの削除がエラーにならないことを検証する。
func TestUserSessionRepository_DeleteByTokenDigest(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)

	builder := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build())
	builder.Build()

	digest := auth.HashToken(builder.Token())

	if err := repo.DeleteByTokenDigest(ctx, digest); err != nil {
		t.Fatalf("DeleteByTokenDigest()のエラー = %v", err)
	}

	session, err := repo.FindLiveWithUserByTokenDigest(ctx, digest)
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
	}
	if session != nil {
		t.Errorf("セッション = %v、期待値 = nil", session)
	}

	if err := repo.DeleteByTokenDigest(ctx, digest); err != nil {
		t.Errorf("削除済みのセッションの再削除のエラー = %v", err)
	}
}

// TestUserSessionRepository_DeleteByUserID は、指定したユーザーのセッションだけが
// すべて消え、他のユーザーのセッションが残ることを検証する。
func TestUserSessionRepository_DeleteByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)

	userID := testutil.NewUserBuilder(t, tx).Build()
	first := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	first.Build()
	second := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID)
	second.Build()

	other := testutil.NewUserSessionBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build())
	other.Build()

	if err := repo.DeleteByUserID(ctx, userID); err != nil {
		t.Fatalf("DeleteByUserID()のエラー = %v", err)
	}

	for _, builder := range []*testutil.UserSessionBuilder{first, second} {
		session, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(builder.Token()))
		if err != nil {
			t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
		}
		if session != nil {
			t.Errorf("セッション = %v、期待値 = nil", session)
		}
	}

	remaining, err := repo.FindLiveWithUserByTokenDigest(ctx, auth.HashToken(other.Token()))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenDigest()のエラー = %v", err)
	}
	if remaining == nil {
		t.Error("他のユーザーのセッション = nil、非nilを期待")
	}
}

// TestUserSessionRepository_DeleteExpired は、有効期限が切れたセッションだけが消え、
// 期限内のセッションが残ることを検証する。
func TestUserSessionRepository_DeleteExpired(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserSessionRepository(db).WithTx(tx)

	now := time.Now().UTC()
	userID := testutil.NewUserBuilder(t, tx).Build()
	expiredID := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID).WithExpiresAt(now.Add(-time.Minute)).Build()
	liveID := testutil.NewUserSessionBuilder(t, tx).WithUserID(userID).WithExpiresAt(now.Add(time.Hour)).Build()

	if err := repo.DeleteExpired(ctx, now); err != nil {
		t.Fatalf("DeleteExpired()のエラー = %v", err)
	}

	tests := []struct {
		name string
		id   model.UserSessionID
		want bool
	}{
		{name: "期限切れのセッション", id: expiredID, want: false},
		{name: "期限内のセッション", id: liveID, want: true},
	}
	for _, tt := range tests {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM user_sessions WHERE id = $1)", uuid.UUID(tt.id)).Scan(&exists); err != nil {
			t.Fatalf("セッションの存在確認のエラー = %v", err)
		}
		if exists != tt.want {
			t.Errorf("%sが残っているか = %t、期待値 = %t", tt.name, exists, tt.want)
		}
	}
}
