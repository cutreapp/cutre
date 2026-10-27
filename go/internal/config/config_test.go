package config

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/clientip"
	"github.com/cutreapp/cutre/go/internal/logging"
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
	t.Setenv("CUTRE_TRUSTED_PROXIES", "")
	t.Setenv("CUTRE_TURNSTILE_SITE_KEY", "")
	t.Setenv("CUTRE_TURNSTILE_SECRET_KEY", "")
	t.Setenv("CUTRE_CONTINUATION_TOKEN_KEY", "test-continuation-token-key-0123456789")
	t.Setenv("CUTRE_TOTP_ENCRYPTION_KEY", "test-totp-encryption-key-0123456789")
	t.Setenv("CUTRE_RESEND_API_KEY", "")
	t.Setenv("CUTRE_EMAIL_FROM", "")
	// 本番 (APP_ENV=prod) はリビジョンが無いと起動しないため、gitコマンドに頼らない固定値を渡す。
	t.Setenv("GIT_REV", "0123456789abcdef")
	t.Setenv("CUTRE_LOG_FORMAT", "")
	t.Setenv("CUTRE_SENTRY_DSN", "")
	t.Setenv("CUTRE_SENTRY_ENVIRONMENT", "")
	t.Setenv("CUTRE_SENTRY_TRACES_SAMPLE_RATE", "")
	t.Setenv("CUTRE_SENTRY_DEBUG", "")
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

// setStampedGitRev はビルド時に埋め込むリビジョンをテストの間だけ差し替える。
// パッケージレベルの変数を書き換えるため、これを使うテストでは t.Parallel() を呼ばない。
func setStampedGitRev(t *testing.T, rev string) {
	t.Helper()

	original := stampedGitRev
	stampedGitRev = rev
	t.Cleanup(func() { stampedGitRev = original })
}

func TestResolveGitRev(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		gitRev  string
		stamped string
		want    string
	}{
		{name: "GIT_REVを埋め込んだ値より優先する", env: "prod", gitRev: "fedcba9876543210", stamped: "0123456789abcdef", want: "fedcba9"},
		{name: "本番ではビルド時に埋め込んだ値を使う", env: "prod", stamped: "0123456789abcdef", want: "0123456"},
		{name: "7文字以下の値はそのまま返す", env: "prod", gitRev: "abc12", want: "abc12"},
		{name: "前後の空白を取り除く", env: "prod", gitRev: " 0123456789 \n", want: "0123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GIT_REV", tt.gitRev)
			setStampedGitRev(t, tt.stamped)

			got, err := resolveGitRev(&Config{Env: tt.env})
			if err != nil {
				t.Fatalf("resolveGitRev()のエラー = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveGitRev() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestLoad_ProductionRequiresGitRev は、本番でリビジョンが取れないときに起動を止めることを検証する。
// アセットバージョンが固定値になり、デプロイしても古いCSS / JSがキャッシュされ続けるのを防ぐため。
func TestLoad_ProductionRequiresGitRev(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("CUTRE_RESEND_API_KEY", "re_test")
	t.Setenv("CUTRE_EMAIL_FROM", "noreply@example.com")
	t.Setenv("GIT_REV", "")
	setStampedGitRev(t, "")

	cfg, err := Load()
	if err == nil {
		t.Fatalf("エラーを期待したが、nilだった (GitRev = %q)", cfg.GitRev)
	}
	if !strings.Contains(err.Error(), "GIT_REV") {
		t.Errorf("エラーメッセージ = %q、GIT_REV を含むことを期待", err.Error())
	}

	setStampedGitRev(t, "fedcba9876543210")

	cfg, err = Load()
	if err != nil {
		t.Fatalf("埋め込んだリビジョンがあるときのLoad()のエラー = %v", err)
	}
	if cfg.GitRev != "fedcba9" {
		t.Errorf("GitRev = %q、期待値 = %q", cfg.GitRev, "fedcba9")
	}
}

// TestLoad_Turnstile は、Turnstileの鍵を両方そろえたときだけ受け付けることを検証する。
func TestLoad_Turnstile(t *testing.T) {
	tests := []struct {
		name      string
		siteKey   string
		secretKey string
		wantErr   bool
	}{
		{name: "両方が空なら無効として起動する", siteKey: "", secretKey: ""},
		{name: "両方を設定すれば読み取る", siteKey: "site-key", secretKey: "secret-key"},
		{name: "サイトキーだけならエラーになる", siteKey: "site-key", secretKey: "", wantErr: true},
		{name: "シークレットキーだけならエラーになる", siteKey: "", secretKey: "secret-key", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("CUTRE_TURNSTILE_SITE_KEY", tt.siteKey)
			t.Setenv("CUTRE_TURNSTILE_SECRET_KEY", tt.secretKey)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("エラーを期待したが、nilだった")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load()のエラー = %v", err)
			}

			if cfg.TurnstileSiteKey != tt.siteKey {
				t.Errorf("TurnstileSiteKey = %q、期待値 = %q", cfg.TurnstileSiteKey, tt.siteKey)
			}
			if cfg.TurnstileSecretKey != tt.secretKey {
				t.Errorf("TurnstileSecretKey = %q、期待値 = %q", cfg.TurnstileSecretKey, tt.secretKey)
			}
		})
	}
}

// TestLoad_TrustedProxies は、前段のプロキシの設定が読み取れる形とその結果を検証する。
func TestLoad_TrustedProxies(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "未設定なら転送ヘッダーを読まない",
			value: "",
			want:  nil,
		},
		{
			name:  "CIDR表記を読み取る",
			value: "172.18.0.0/16",
			want:  []string{"172.18.0.0/16"},
		},
		{
			name:  "単体のアドレスはそのアドレスだけの範囲として扱う",
			value: "203.0.113.1",
			want:  []string{"203.0.113.1/32"},
		},
		{
			name:  "単体のIPv6アドレスも同じく1つのアドレスの範囲にする",
			value: "2001:db8::1",
			want:  []string{"2001:db8::1/128"},
		},
		{
			name:  "カンマ区切りで複数の前段を設定できる",
			value: " 172.18.0.0/16 , 2001:db8::/32 ",
			want:  []string{"172.18.0.0/16", "2001:db8::/32"},
		},
		{
			name:  "ホスト部を持つCIDRはネットワークへ丸める",
			value: "172.18.0.5/16",
			want:  []string{"172.18.0.0/16"},
		},
		{
			name:  "IPv4射影アドレスをIPv4に揃える",
			value: "::ffff:172.18.0.1",
			want:  []string{"172.18.0.1/32"},
		},
		{
			name:  "IPv4射影CIDRをIPv4に揃えてホスト部を丸める",
			value: "::ffff:172.18.0.5/112",
			want:  []string{"172.18.0.0/16"},
		},
		{
			name:  "IPv4射影CIDRの最小の範囲は単体のIPv4になる",
			value: "::ffff:172.18.0.1/128",
			want:  []string{"172.18.0.1/32"},
		},
		{
			name:  "IPv4射影CIDRの最大の範囲はIPv4全体になる",
			value: "::ffff:0.0.0.0/96",
			want:  []string{"0.0.0.0/0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("CUTRE_TRUSTED_PROXIES", tt.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load()のエラー = %v", err)
			}

			if len(cfg.TrustedProxies) != len(tt.want) {
				t.Fatalf("TrustedProxiesの数 = %d、期待値 = %d", len(cfg.TrustedProxies), len(tt.want))
			}
			for i, want := range tt.want {
				if got := cfg.TrustedProxies[i].String(); got != want {
					t.Errorf("TrustedProxies[%d] = %q、期待値 = %q", i, got, want)
				}
			}
		})
	}
}

// TestLoad_InvalidTrustedProxies は、読み取れない設定値で起動を止めることを検証する。
// 黙って無視すると、設定したつもりの前段が信頼されないまま動き続けることになる。
func TestLoad_InvalidTrustedProxies(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "アドレスでもCIDRでもない値", value: "proxy.example.com"},
		{name: "範囲の指定が壊れている値", value: "172.18.0.0/99"},
		{name: "正しい値に混ざった不正な値", value: "172.18.0.0/16,not-an-address"},
		{name: "IPv4射影範囲の外を含むCIDR", value: "::ffff:172.18.0.1/95"},
		// IPv6のまま持つと、IPv4へ展開して照合する接続元には一致しない。
		{name: "IPv4射影範囲を含むIPv6全体", value: "::/0"},
		{name: "IPv4射影範囲を含むIPv6のCIDR", value: "::/80"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("CUTRE_TRUSTED_PROXIES", tt.value)

			if _, err := Load(); err == nil {
				t.Error("エラーを期待したが、nilだった")
			}
		})
	}
}

// TestLoad_TrustedProxies_Resolve は、設定の表記を変えても信頼する範囲とクライアントIPが変わらないことを検証する。
func TestLoad_TrustedProxies_Resolve(t *testing.T) {
	type remote struct {
		addr string
		want string
	}

	tests := []struct {
		name    string
		values  []string
		remotes []remote
	}{
		{
			name:   "単体のアドレス",
			values: []string{"172.18.0.1", "::ffff:172.18.0.1", "::ffff:172.18.0.1/128"},
			remotes: []remote{
				{addr: "172.18.0.1:54321", want: "203.0.113.10"},
				{addr: "[::ffff:172.18.0.1]:54321", want: "203.0.113.10"},
				{addr: "172.18.0.2:54321", want: "172.18.0.2"},
			},
		},
		{
			name:   "ネットワーク",
			values: []string{"172.18.0.0/16", "::ffff:172.18.0.5/112"},
			remotes: []remote{
				{addr: "172.18.0.1:54321", want: "203.0.113.10"},
				{addr: "[::ffff:172.18.0.1]:54321", want: "203.0.113.10"},
				{addr: "172.19.0.1:54321", want: "172.19.0.1"},
			},
		},
	}

	for _, tt := range tests {
		for _, value := range tt.values {
			t.Run(tt.name+"/"+value, func(t *testing.T) {
				setRequiredEnv(t)
				t.Setenv("CUTRE_TRUSTED_PROXIES", value)

				cfg, err := Load()
				if err != nil {
					t.Fatalf("Load()のエラー = %v", err)
				}

				for _, r := range tt.remotes {
					req := httptest.NewRequest(http.MethodGet, "/", nil)
					req.RemoteAddr = r.addr
					req.Header.Set("X-Forwarded-For", "203.0.113.10")

					if got := clientip.Resolve(req, cfg.TrustedProxies); got != r.want {
						t.Errorf("接続元=%qのResolve() = %q、期待値 = %q", r.addr, got, r.want)
					}
				}
			})
		}
	}
}

// TestLoad_Email は、メールの送信の設定を読み取れる組み合わせと、起動を止める組み合わせを検証する。
func TestLoad_Email(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		apiKey  string
		from    string
		wantErr bool
	}{
		{name: "本番以外でAPIキーが空ならログへ出力する設定として起動する", env: "dev"},
		{name: "APIキーと送信元を設定すれば読み取る", env: "prod", apiKey: "re_test", from: "noreply@cutre.example.com"},
		{name: "本番でAPIキーが空ならエラーになる", env: "prod", wantErr: true},
		{name: "APIキーだけならエラーになる", env: "dev", apiKey: "re_test", wantErr: true},
		{name: "送信元がメールアドレスとして読めなければエラーになる", env: "dev", apiKey: "re_test", from: "noreply", wantErr: true},
		{name: "送信元に表示名を付けるとエラーになる", env: "dev", apiKey: "re_test", from: "Cutre <noreply@cutre.example.com>", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_ENV", tt.env)
			t.Setenv("CUTRE_RESEND_API_KEY", tt.apiKey)
			t.Setenv("CUTRE_EMAIL_FROM", tt.from)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("エラーを期待したが、nilだった")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load()のエラー = %v", err)
			}

			if cfg.ResendAPIKey != tt.apiKey {
				t.Errorf("ResendAPIKey = %q、期待値 = %q", cfg.ResendAPIKey, tt.apiKey)
			}
			if cfg.EmailFrom != tt.from {
				t.Errorf("EmailFrom = %q、期待値 = %q", cfg.EmailFrom, tt.from)
			}
		})
	}
}

// TestLoad_ContinuationTokenKey は、継続トークンの鍵が無い / 短いときに起動を止めることを検証する。
func TestLoad_ContinuationTokenKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "32バイトの鍵を読み取る", key: strings.Repeat("k", ContinuationTokenMinimumKeyLength)},
		{name: "空ならエラーになる", key: "", wantErr: true},
		{name: "31バイトならエラーになる", key: strings.Repeat("k", ContinuationTokenMinimumKeyLength-1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("CUTRE_CONTINUATION_TOKEN_KEY", tt.key)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("エラーを期待したが、nilだった")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load()のエラー = %v", err)
			}
			if cfg.ContinuationTokenKey != tt.key {
				t.Errorf("ContinuationTokenKey = %q、期待値 = %q", cfg.ContinuationTokenKey, tt.key)
			}
		})
	}
}

// TestLoad_TOTPEncryptionKey は、TOTPの秘密鍵を守る鍵が無い / 短いときに起動を止めることを検証する。
func TestLoad_TOTPEncryptionKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "32バイトの鍵を読み取る", key: strings.Repeat("k", TOTPEncryptionMinimumKeyLength)},
		{name: "空ならエラーになる", key: "", wantErr: true},
		{name: "31バイトならエラーになる", key: strings.Repeat("k", TOTPEncryptionMinimumKeyLength-1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("CUTRE_TOTP_ENCRYPTION_KEY", tt.key)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("エラーを期待したが、nilだった")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load()のエラー = %v", err)
			}
			if cfg.TOTPEncryptionKey != tt.key {
				t.Errorf("TOTPEncryptionKey = %q、期待値 = %q", cfg.TOTPEncryptionKey, tt.key)
			}
		})
	}
}

// TestLoad_LogFormat は、ログの出力形式の読み取りと既定値、読めない値で起動を止めることを検証する。
func TestLoad_LogFormat(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    logging.Format
		wantErr bool
	}{
		{name: "未設定ならText", value: "", want: logging.FormatText},
		{name: "textを読み取る", value: "text", want: logging.FormatText},
		{name: "jsonを読み取る", value: "json", want: logging.FormatJSON},
		{name: "未知の形式はエラーになる", value: "xml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("CUTRE_LOG_FORMAT", tt.value)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("エラーを期待したが、nilだった")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load()のエラー = %v", err)
			}

			if cfg.LogFormat != tt.want {
				t.Errorf("LogFormat = %q、期待値 = %q", cfg.LogFormat, tt.want)
			}
		})
	}
}

func TestLoad_Sentry(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CUTRE_SENTRY_DSN", "https://public@o0.ingest.sentry.io/1")
	t.Setenv("CUTRE_SENTRY_ENVIRONMENT", "production")
	t.Setenv("CUTRE_SENTRY_TRACES_SAMPLE_RATE", "0.1")
	t.Setenv("CUTRE_SENTRY_DEBUG", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()のエラー = %v", err)
	}

	if cfg.SentryDSN != "https://public@o0.ingest.sentry.io/1" {
		t.Errorf("SentryDSN = %q、期待値 = %q", cfg.SentryDSN, "https://public@o0.ingest.sentry.io/1")
	}
	if cfg.SentryEnvironment != "production" {
		t.Errorf("SentryEnvironment = %q、期待値 = %q", cfg.SentryEnvironment, "production")
	}
	if cfg.SentryTracesSampleRate != 0.1 {
		t.Errorf("SentryTracesSampleRate = %v、期待値 = %v", cfg.SentryTracesSampleRate, 0.1)
	}
	if !cfg.SentryDebug {
		t.Error("SentryDebug = false、期待値 = true")
	}
}

func TestLoad_SentryDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()のエラー = %v", err)
	}

	if cfg.SentryDSN != "" {
		t.Errorf("SentryDSN = %q、期待値 = 空", cfg.SentryDSN)
	}
	if cfg.SentryEnvironment != "test" {
		t.Errorf("SentryEnvironment = %q、期待値 = APP_ENV の値 (%q)", cfg.SentryEnvironment, "test")
	}
	if cfg.SentryTracesSampleRate != 0.5 {
		t.Errorf("SentryTracesSampleRate = %v、期待値 = %v", cfg.SentryTracesSampleRate, 0.5)
	}
	if cfg.SentryDebug {
		t.Error("SentryDebug = true、期待値 = false")
	}
}

func TestLoad_InvalidSentry(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "数でない割合", key: "CUTRE_SENTRY_TRACES_SAMPLE_RATE", value: "half"},
		{name: "0未満の割合", key: "CUTRE_SENTRY_TRACES_SAMPLE_RATE", value: "-0.1"},
		{name: "1を超える割合", key: "CUTRE_SENTRY_TRACES_SAMPLE_RATE", value: "1.5"},
		{name: "NaNの割合", key: "CUTRE_SENTRY_TRACES_SAMPLE_RATE", value: "NaN"},
		{name: "真偽値でないデバッグ", key: "CUTRE_SENTRY_DEBUG", value: "yes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(tt.key, tt.value)

			if _, err := Load(); err == nil {
				t.Fatal("エラーを期待したが、nilだった")
			}
		})
	}
}
