package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestRunInvitation_Usage は、操作が無い / 未知のときに使い方を表示して失敗することを検証する。
func TestRunInvitation_Usage(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{}, {"delete"}, {"create", "extra"}} {
		var stdout, stderr bytes.Buffer

		if code := runInvitation(args, &stdout, &stderr); code != exitUsage {
			t.Errorf("runInvitation(%q)の終了コード = %d、期待値 = %d", args, code, exitUsage)
		}
		if !strings.Contains(stderr.String(), "使い方: cutre invitation <操作>") {
			t.Errorf("runInvitation(%q)の標準エラー出力 = %q、使い方を期待", args, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("runInvitation(%q)の標準出力 = %q、空を期待", args, stdout.String())
		}
	}
}

// setCommandEnv は、データベースに接続するサブコマンドが config.Load で読む環境変数をテスト用の値にする。
// t.Setenv を使うため、呼び出すテストは t.Parallel() を呼べない。
func setCommandEnv(t *testing.T) {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL が設定されていません。make test で実行してください")
	}
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("CUTRE_PORT", "8080")
	t.Setenv("CUTRE_DOMAIN", "cutre.example.com")
	t.Setenv("CUTRE_TRUSTED_PROXIES", "")
	t.Setenv("CUTRE_TURNSTILE_SITE_KEY", "")
	t.Setenv("CUTRE_TURNSTILE_SECRET_KEY", "")
	t.Setenv("CUTRE_CONTINUATION_TOKEN_KEY", "test-continuation-token-key-0123456789")
	t.Setenv("CUTRE_TOTP_ENCRYPTION_KEY", "test-totp-encryption-key-0123456789")
	t.Setenv("CUTRE_RESEND_API_KEY", "")
	t.Setenv("CUTRE_EMAIL_FROM", "")
}

// TestRun_InvitationCreate は、発行した招待のリンクを標準出力へ1行で書き、招待がデータベースに残ることを検証する。
// t.Setenv を使うため t.Parallel() は呼ばない。
func TestRun_InvitationCreate(t *testing.T) {
	setCommandEnv(t)

	var stdout bytes.Buffer
	if code := runInvitationCreate(&stdout); code != 0 {
		t.Fatalf("終了コード = %d、期待値 = 0", code)
	}

	const prefix = "https://cutre.example.com/i/"
	line := stdout.String()
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "\n") || strings.Count(line, "\n") != 1 {
		t.Fatalf("標準出力 = %q、%s で始まる1行を期待", line, prefix)
	}

	token := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "\n")
	var inviterIsNull bool
	if err := testutil.GetTestDB().QueryRowContext(context.Background(),
		"SELECT inviter_user_id IS NULL FROM invitations WHERE token = $1", token).Scan(&inviterIsNull); err != nil {
		t.Fatalf("招待の取得のエラー = %v", err)
	}
	if !inviterIsNull {
		t.Error("招待者が記録されている、管理者が発行した招待 (招待者無し) を期待")
	}
}
