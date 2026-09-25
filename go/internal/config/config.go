// Package config は環境変数からアプリケーションの設定を読み込む。
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ContinuationTokenMinimumKeyLength は継続トークンの鍵に求める最小のバイト数。
// HMAC-SHA-256の出力と同じ32バイトを下回ると、鍵の総当たりが署名の偽造より易しくなる。
const ContinuationTokenMinimumKeyLength = 32

// TOTPEncryptionMinimumKeyLength はTOTPの秘密鍵を守る鍵に求める最小のバイト数。
// 導く鍵 (AES-256・HMAC-SHA-256) と同じ32バイトを下回ると、鍵の総当たりが暗号を破るより易しくなる。
const TOTPEncryptionMinimumKeyLength = 32

// Config はアプリケーションの設定を保持する。
type Config struct {
	// Env は実行環境 (dev / test / prod)。
	Env string

	DatabaseURL string

	Port   string
	Domain string

	// TrustedProxies はアプリの前段に立つリバースプロキシが接続してくるネットワーク。
	//
	// ここに挙げたアドレスから届いたリクエストは、運んできた X-Forwarded-For から
	// クライアントのアドレスを読み取る。挙げていないアドレスから届いたリクエストは、
	// 接続元のアドレスをクライアントとして扱う (internal/clientip)。
	//
	// 既定は空で、そのときは転送ヘッダーを一切読まない。
	// 前段が複数あるときは最も近い1つだけでなくすべてを設定する。
	TrustedProxies []netip.Prefix

	// TurnstileSiteKey と TurnstileSecretKey はCloudflare TurnstileによるBot対策の鍵。
	// サイトキーはウィジェットを描画するテンプレートへ渡し、シークレットキーは送信された
	// トークンをサーバー側で検証するのに使う (internal/turnstile)。
	//
	// 両方が空のときはTurnstileを無効にし、ウィジェットを描画せず検証も通す。
	// 開発・テスト環境はこの状態で動かす。
	TurnstileSiteKey   string
	TurnstileSecretKey string

	// ContinuationTokenKey は、登録の途中の状態 (受け取った招待など) を運ぶCookieに署名する鍵。
	// 変えると発行済みのCookieが無効になり、漏れると任意の招待で登録を始められるCookieを作られる。
	ContinuationTokenKey string

	// TOTPEncryptionKey は、TOTPの秘密鍵の暗号化とリカバリーコードのダイジェストに使う鍵 (internal/auth.TwoFactorKey)。
	// 変えると二要素認証を有効にしているユーザーがログインできなくなり、漏れるとデータベースと合わせて秘密鍵を読まれる。
	TOTPEncryptionKey string

	// ResendAPIKey はメールの送信に使うResendのAPIキー。
	// 空のときは送らずにログへ出力する (internal/email)。本番では空を許さない。
	ResendAPIKey string

	// EmailFrom は送信するメールのFromに使うアドレス。ResendAPIKey を設定するときは必須。
	EmailFrom string

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

	trustedProxies, err := parseTrustedProxies(os.Getenv("CUTRE_TRUSTED_PROXIES"))
	if err != nil {
		return nil, err
	}
	cfg.TrustedProxies = trustedProxies

	// 片方だけの設定は起動を止める。サイトキーだけならウィジェットは出るのに検証は素通りし、
	// シークレットキーだけならトークンが届かずすべての送信が弾かれる。どちらも黙って動き続けると
	// Bot対策が効いていないこと・フォームが使えないことに気付きにくい。
	cfg.TurnstileSiteKey = os.Getenv("CUTRE_TURNSTILE_SITE_KEY")
	cfg.TurnstileSecretKey = os.Getenv("CUTRE_TURNSTILE_SECRET_KEY")
	if (cfg.TurnstileSiteKey == "") != (cfg.TurnstileSecretKey == "") {
		return nil, errors.New("環境変数 CUTRE_TURNSTILE_SITE_KEY と CUTRE_TURNSTILE_SECRET_KEY は両方を設定するか、両方を空にしてください")
	}

	cfg.ContinuationTokenKey = os.Getenv("CUTRE_CONTINUATION_TOKEN_KEY")
	if len(cfg.ContinuationTokenKey) < ContinuationTokenMinimumKeyLength {
		return nil, fmt.Errorf("環境変数 CUTRE_CONTINUATION_TOKEN_KEY に%dバイト以上の値を設定してください", ContinuationTokenMinimumKeyLength)
	}

	cfg.TOTPEncryptionKey = os.Getenv("CUTRE_TOTP_ENCRYPTION_KEY")
	if len(cfg.TOTPEncryptionKey) < TOTPEncryptionMinimumKeyLength {
		return nil, fmt.Errorf("環境変数 CUTRE_TOTP_ENCRYPTION_KEY に%dバイト以上の値を設定してください", TOTPEncryptionMinimumKeyLength)
	}

	if err := loadEmail(cfg); err != nil {
		return nil, err
	}

	cfg.GitRev = getGitCommitHash()

	return cfg, nil
}

// loadEmail はメールの送信の設定を読み取る。
//
// 本番でAPIキーが空だとメールがログへ出力されるだけになり、確認コードやパスワードリセットのメールが
// 届かないまま動き続けるため、起動を止める。
// 送信元のアドレスは、読めない値のまま送るとResendがすべての送信を拒むため、起動時に確かめる。
func loadEmail(cfg *Config) error {
	cfg.ResendAPIKey = os.Getenv("CUTRE_RESEND_API_KEY")
	cfg.EmailFrom = os.Getenv("CUTRE_EMAIL_FROM")

	if cfg.ResendAPIKey == "" {
		if cfg.IsProduction() {
			return errors.New("本番環境では環境変数 CUTRE_RESEND_API_KEY を設定してください")
		}
		return nil
	}

	if cfg.EmailFrom == "" {
		return errors.New("環境変数 CUTRE_RESEND_API_KEY を設定するときは CUTRE_EMAIL_FROM も設定してください")
	}
	// 表示名付きの形 (Cutre <noreply@example.com>) はParseAddressが受け付けてしまうため、読み取った結果と比べる。
	if addr, err := mail.ParseAddress(cfg.EmailFrom); err != nil || addr.Address != cfg.EmailFrom {
		return fmt.Errorf("環境変数 CUTRE_EMAIL_FROM にはメールアドレスだけを設定してください: %q", cfg.EmailFrom)
	}

	return nil
}

// ipv4MappedRange はIPv4アドレスをIPv6で表すときに使うIPv4射影アドレスの範囲。
var ipv4MappedRange = netip.MustParsePrefix("::ffff:0:0/96")

// parseTrustedProxies は信頼するプロキシのネットワークをカンマ区切りの設定値から読み取る。
//
// 受け付けるのはCIDR表記 (`10.0.0.0/8`) と単体のアドレス (`203.0.113.1`) の2つ。
// 単体のアドレスはそのアドレスだけを含む範囲として扱い、運用者が1台のプロキシを
// わざわざ /32 や /128 と書かずに済むようにする。
//
// 読めない値は起動を止める。黙って無視すると、設定したつもりのプロキシが信頼されず、
// 訪問者のアドレスが前段のものに置き換わったことに気付けないため。
func parseTrustedProxies(value string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix

	for entry := range strings.SplitSeq(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Addr().Is6() && prefix.Overlaps(ipv4MappedRange) {
				// 接続元のIPはIPv4へ展開して照合するため、設定側も同じアドレス族に揃える。
				// IPv4射影範囲の外まで含むCIDR (::/0 など) は1つのIPv4の範囲へ変換できず、
				// IPv6のまま持つとIPv4射影形式の接続元に一致しないため、分けて書くよう求める。
				if prefix.Bits() < ipv4MappedRange.Bits() {
					return nil, fmt.Errorf("環境変数 CUTRE_TRUSTED_PROXIES のCIDRがIPv4射影範囲 (%s) の外まで含んでいます。IPv4の範囲とIPv6の範囲に分けて指定してください: %q", ipv4MappedRange, entry)
				}
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-ipv4MappedRange.Bits())
			}
			prefixes = append(prefixes, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("環境変数 CUTRE_TRUSTED_PROXIES に読み取れない値が含まれています: %q", entry)
		}

		addr = addr.Unmap()
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}

	return prefixes, nil
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
