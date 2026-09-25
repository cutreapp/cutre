package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

// TestRunSeed_RejectsNonDev は、名簿やDBを読むより前に環境のガードで拒否することを確かめる。
// 環境変数と既定のロガーを変更するため、並行実行しない。
func TestRunSeed_RejectsNonDev(t *testing.T) {
	for _, env := range []string{"prod", "test"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			t.Setenv("DATABASE_URL", "postgres://unused.invalid/cutre")
			t.Setenv("CUTRE_PORT", "8080")
			t.Setenv("CUTRE_DOMAIN", "cutre.example.com")
			t.Setenv("CUTRE_TRUSTED_PROXIES", "")
			t.Setenv("CUTRE_TURNSTILE_SITE_KEY", "")
			t.Setenv("CUTRE_TURNSTILE_SECRET_KEY", "")
			t.Setenv("CUTRE_CONTINUATION_TOKEN_KEY", "test-continuation-token-key-0123456789")
			t.Setenv("CUTRE_TOTP_ENCRYPTION_KEY", "test-totp-encryption-key-0123456789")
			// 本番ではメールの送信の設定が必須のため、設定の読み込みで止まらないよう渡しておく。
			t.Setenv("CUTRE_RESEND_API_KEY", "re_test")
			t.Setenv("CUTRE_EMAIL_FROM", "noreply@cutre.example.com")
			t.Setenv("GIT_REV", "test")
			// 名簿のない場所でも、ファイルのエラーではなく環境の拒否が先に起きる。
			t.Chdir(t.TempDir())
			var logs bytes.Buffer
			oldLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(oldLogger) })
			var stdout bytes.Buffer
			if code := runSeed(&stdout); code != 1 {
				t.Fatalf("終了コード = %d、期待値 = 1", code)
			}
			decoder := json.NewDecoder(&logs)
			var entry map[string]any
			if err := decoder.Decode(&entry); err != nil {
				t.Fatalf("ログの読み込みに失敗: %v", err)
			}
			if entry["msg"] != "seed は開発環境でしか実行できません" || entry["env"] != env {
				t.Errorf("拒否理由 = %v、環境 %s の拒否を期待", entry, env)
			}
			if err := decoder.Decode(&entry); err != io.EOF {
				t.Errorf("後続のログがある: %v", err)
			}
			if stdout.Len() != 0 {
				t.Errorf("標準出力 = %q、空を期待", stdout.String())
			}
		})
	}
}
