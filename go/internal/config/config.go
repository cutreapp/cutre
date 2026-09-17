// Package config は環境変数からアプリケーションの設定を読み込む。
package config

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Config はアプリケーションの設定を保持する。
type Config struct {
	// Env は実行環境 (dev / test / prod)。
	Env string

	DatabaseURL string

	Port   string
	Domain string

	// GitRev はデプロイしたGitコミットの短縮ハッシュ。本番 / テストのアセットバージョンに使う。
	GitRev string
}

// Load は環境変数から設定を読み込む。
// 必須の環境変数が無い場合はエラーを返し、起動を止める。
//
// 環境変数はプロセスの起動前に注入されている前提とする。
// ローカルでは `op run --env-file=.env`、CIではGitHub Actions、本番ではDokkuが設定する。
func Load() (*Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	cfg := &Config{
		Env: env,
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, errors.New("必須の環境変数 DATABASE_URL が設定されていません")
	}

	cfg.Port = os.Getenv("CUTRE_PORT")
	if cfg.Port == "" {
		return nil, errors.New("必須の環境変数 CUTRE_PORT が設定されていません")
	}

	cfg.Domain = os.Getenv("CUTRE_DOMAIN")
	if cfg.Domain == "" {
		return nil, errors.New("必須の環境変数 CUTRE_DOMAIN が設定されていません")
	}

	cfg.GitRev = getGitCommitHash()

	return cfg, nil
}

// IsDev は開発環境かどうかを返す。
func (c *Config) IsDev() bool {
	return c.Env == "dev"
}

// IsTest はテスト環境かどうかを返す。
func (c *Config) IsTest() bool {
	return c.Env == "test"
}

// IsProduction は本番環境かどうかを返す。
func (c *Config) IsProduction() bool {
	return c.Env == "prod"
}

// AppURL はアプリケーションのベースURLを返す。
func (c *Config) AppURL() string {
	return "https://" + c.Domain
}

// AssetVersion はCSS / JSのURLに付けるバージョン文字列を返す。
// 開発環境では編集をすぐに反映させるため、呼び出しごとに変わるミリ秒のタイムスタンプを返す。
// それ以外の環境では、デプロイ単位でキャッシュを切り替えられるようGitコミットの短縮ハッシュを返す。
func (c *Config) AssetVersion() string {
	if c.IsDev() {
		return strconv.FormatInt(time.Now().UnixMilli(), 10)
	}
	return c.GitRev
}

// getGitCommitHash は実行中のビルドのGitコミットの短縮ハッシュを返す。
//
// Dokkuのデプロイ先コンテナには .git ディレクトリが無く `git rev-parse` が失敗するため、
// Dokkuがデプロイ時のコミットハッシュを渡す GIT_REV を最優先する。
// プラットフォームが提供する変数のため CUTRE_ プレフィックスは付かない。
// ローカルのgitコマンドは開発用のフォールバックで、どちらも使えなければ "dev" を返す。
func getGitCommitHash() string {
	if rev := strings.TrimSpace(os.Getenv("GIT_REV")); rev != "" {
		// `git rev-parse --short` の短縮形とおおよそ揃える。
		const shortHashLen = 7
		if len(rev) > shortHashLen {
			return rev[:shortHashLen]
		}
		return rev
	}

	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(out))
}
