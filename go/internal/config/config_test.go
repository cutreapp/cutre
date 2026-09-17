package config

import (
	"strconv"
	"testing"
)

// t.Setenv はプロセス全体の環境変数を書き換えるため、t.Setenv を使うテストでは t.Parallel() を呼ばない。

// setRequiredEnv は必須の環境変数をテスト用の値で設定する。
// `op run` 経由で実行したときに .env の値がテストへ混入しないよう、すべて上書きする。
func setRequiredEnv(t *testing.T) {
	t.Helper()

	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres@localhost:5432/cutre_test?sslmode=disable")
	t.Setenv("CUTRE_PORT", "8080")
	t.Setenv("CUTRE_DOMAIN", "cutre.example.com")
	t.Setenv("GIT_REV", "")
}

func TestLoad(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()のエラー = %v", err)
	}

	if cfg.Env != "test" {
		t.Errorf("Env = %q、期待値 = %q", cfg.Env, "test")
	}
	if cfg.DatabaseURL != "postgres://postgres@localhost:5432/cutre_test?sslmode=disable" {
		t.Errorf("DatabaseURL = %q、期待値 = %q", cfg.DatabaseURL, "postgres://postgres@localhost:5432/cutre_test?sslmode=disable")
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q、期待値 = %q", cfg.Port, "8080")
	}
	if cfg.Domain != "cutre.example.com" {
		t.Errorf("Domain = %q、期待値 = %q", cfg.Domain, "cutre.example.com")
	}
}

func TestLoad_DefaultsEnvToDev(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()のエラー = %v", err)
	}

	if cfg.Env != "dev" {
		t.Errorf("Env = %q、期待値 = %q", cfg.Env, "dev")
	}
}

func TestLoad_MissingRequiredEnv(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "DATABASE_URLが無いとエラーになる", key: "DATABASE_URL"},
		{name: "CUTRE_PORTが無いとエラーになる", key: "CUTRE_PORT"},
		{name: "CUTRE_DOMAINが無いとエラーになる", key: "CUTRE_DOMAIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(tt.key, "")

			cfg, err := Load()
			if err == nil {
				t.Fatal("エラーを期待したが、nilだった")
			}
			if cfg != nil {
				t.Errorf("Config = %+v、nilを期待", cfg)
			}
			want := "必須の環境変数 " + tt.key + " が設定されていません"
			if err.Error() != want {
				t.Errorf("エラーメッセージ = %q、期待値 = %q", err.Error(), want)
			}
		})
	}
}

func TestConfig_EnvPredicates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		env            string
		wantDev        bool
		wantTest       bool
		wantProduction bool
	}{
		{env: "dev", wantDev: true},
		{env: "test", wantTest: true},
		{env: "prod", wantProduction: true},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{Env: tt.env}

			if got := cfg.IsDev(); got != tt.wantDev {
				t.Errorf("IsDev() = %v、期待値 = %v", got, tt.wantDev)
			}
			if got := cfg.IsTest(); got != tt.wantTest {
				t.Errorf("IsTest() = %v、期待値 = %v", got, tt.wantTest)
			}
			if got := cfg.IsProduction(); got != tt.wantProduction {
				t.Errorf("IsProduction() = %v、期待値 = %v", got, tt.wantProduction)
			}
		})
	}
}

func TestConfig_AppURL(t *testing.T) {
	t.Parallel()

	cfg := &Config{Domain: "cutre.example.com"}

	if got := cfg.AppURL(); got != "https://cutre.example.com" {
		t.Errorf("AppURL() = %q、期待値 = %q", got, "https://cutre.example.com")
	}
}

func TestConfig_AssetVersion(t *testing.T) {
	t.Parallel()

	t.Run("開発環境ではミリ秒のタイムスタンプを返す", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Env: "dev", GitRev: "abc1234"}

		got := cfg.AssetVersion()
		if _, err := strconv.ParseInt(got, 10, 64); err != nil {
			t.Errorf("AssetVersion() = %q、数値を期待", got)
		}
	})

	for _, env := range []string{"test", "prod"} {
		t.Run(env+"ではGitRevを返す", func(t *testing.T) {
			t.Parallel()

			cfg := &Config{Env: env, GitRev: "abc1234"}

			if got := cfg.AssetVersion(); got != "abc1234" {
				t.Errorf("AssetVersion() = %q、期待値 = %q", got, "abc1234")
			}
		})
	}
}

func TestGetGitCommitHash_GitRev(t *testing.T) {
	tests := []struct {
		name   string
		gitRev string
		want   string
	}{
		{name: "7文字より長い値は先頭7文字に短縮する", gitRev: "0123456789abcdef", want: "0123456"},
		{name: "7文字以下の値はそのまま返す", gitRev: "abc12", want: "abc12"},
		{name: "前後の空白を取り除く", gitRev: " 0123456789 \n", want: "0123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GIT_REV", tt.gitRev)

			if got := getGitCommitHash(); got != tt.want {
				t.Errorf("getGitCommitHash() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}
