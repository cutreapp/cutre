package repository_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestCreateInvitationRedemptions_Backfill は、旧スキーマの使用済み招待を新しい使用記録へ移し、
// 未使用の招待には記録を作らないことを検証する。
func TestCreateInvitationRedemptions_Backfill(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	ctx := context.Background()

	// 共有テストDBの現行テーブルと名前が重ならないよう、旧スキーマをトランザクション内に作る。
	schema := "invitation_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := tx.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("検証用スキーマの作成のエラー = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path TO "+schema+", public"); err != nil {
		t.Fatalf("検索パスの設定のエラー = %v", err)
	}
	if _, err := tx.ExecContext(ctx, "CREATE TABLE users (id uuid PRIMARY KEY)"); err != nil {
		t.Fatalf("検証用ユーザーテーブルの作成のエラー = %v", err)
	}
	applyMigrationUp(t, tx, "../../db/migrations/20260924060023_create_invitations.sql")

	usedUserID := uuid.New()
	usedInvitationID := uuid.New()
	unusedInvitationID := uuid.New()
	usedAt := time.Date(2026, time.September, 24, 15, 4, 5, 123456000, time.UTC)
	if _, err := tx.ExecContext(ctx, "INSERT INTO users (id) VALUES ($1)", usedUserID); err != nil {
		t.Fatalf("利用者の作成のエラー = %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO invitations (id, token, expires_at, used_by_user_id, used_at) VALUES ($1, $2, $3, $4, $5)",
		usedInvitationID, "used-admin-invitation", usedAt.Add(24*time.Hour), usedUserID, usedAt,
	); err != nil {
		t.Fatalf("使用済み招待の作成のエラー = %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO invitations (id, token, expires_at) VALUES ($1, $2, $3)",
		unusedInvitationID, "unused-admin-invitation", usedAt.Add(24*time.Hour),
	); err != nil {
		t.Fatalf("未使用の招待の作成のエラー = %v", err)
	}

	applyMigrationUp(t, tx, "../../db/migrations/20260925035127_create_invitation_redemptions.sql")

	var gotInvitationID, gotUserID uuid.UUID
	var createdAt, updatedAt time.Time
	if err := tx.QueryRowContext(ctx,
		"SELECT invitation_id, user_id, created_at, updated_at FROM invitation_redemptions",
	).Scan(&gotInvitationID, &gotUserID, &createdAt, &updatedAt); err != nil {
		t.Fatalf("移行した使用記録の取得のエラー = %v", err)
	}
	if gotInvitationID != usedInvitationID || gotUserID != usedUserID || !createdAt.Equal(usedAt) || !updatedAt.Equal(usedAt) {
		t.Errorf("移行した使用記録 = (%v, %v, %v, %v)、期待値 = (%v, %v, %v, %v)",
			gotInvitationID, gotUserID, createdAt, updatedAt, usedInvitationID, usedUserID, usedAt, usedAt)
	}

	var unusedCount int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM invitation_redemptions WHERE invitation_id = $1", unusedInvitationID,
	).Scan(&unusedCount); err != nil {
		t.Fatalf("未使用の招待の使用記録の件数の取得のエラー = %v", err)
	}
	if unusedCount != 0 {
		t.Errorf("未使用の招待の使用記録の件数 = %d、期待値 = 0", unusedCount)
	}
}

// applyMigrationUp は指定したマイグレーションのup側を実行する。
// このテストで使う2ファイルには、SQL文字列内のセミコロンが無い。
func applyMigrationUp(t *testing.T, tx *sql.Tx, name string) {
	t.Helper()

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%sの読み取りのエラー = %v", name, err)
	}
	up, _, ok := strings.Cut(string(data), "-- migrate:down")
	if !ok {
		t.Fatalf("%sにdownの区切りが無い", name)
	}
	for _, statement := range strings.Split(up, ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := tx.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("%sのupの実行のエラー = %v (SQL: %s)", name, err, statement)
		}
	}
}
