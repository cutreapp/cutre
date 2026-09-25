package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/cutreapp/cutre/go/internal/clientip"
)

// prefixes は信頼するプロキシのネットワークをテスト用に組み立てる。
func prefixes(t *testing.T, values ...string) []netip.Prefix {
	t.Helper()

	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			t.Fatalf("テスト用のネットワークの組み立てに失敗しました: %v", err)
		}
		result = append(result, prefix)
	}

	return result
}

// TestResolve は、転送ヘッダーを信じてよい場面と信じてはいけない場面で、
// クライアントとして採るアドレスが変わることを検証する。
func TestResolve(t *testing.T) {
	t.Parallel()

	trusted := prefixes(t, "172.18.0.0/16", "2001:db8::/32")

	tests := []struct {
		name           string
		remoteAddr     string
		forwardedFor   []string
		trustedProxies []netip.Prefix
		want           string
	}{
		{
			name:       "信頼するプロキシを設定していなければ接続元を使う",
			remoteAddr: "172.18.0.1:54321",
			// 前段が無いアプリに届く転送ヘッダーはクライアントが書いた値に過ぎない。
			forwardedFor: []string{"203.0.113.10"},
			want:         "172.18.0.1",
		},
		{
			name:           "信頼しない接続元の転送ヘッダーは読まない",
			remoteAddr:     "198.51.100.7:54321",
			forwardedFor:   []string{"203.0.113.10"},
			trustedProxies: trusted,
			want:           "198.51.100.7",
		},
		{
			name:           "信頼するプロキシ経由なら転送ヘッダーのアドレスを使う",
			remoteAddr:     "172.18.0.1:54321",
			forwardedFor:   []string{"203.0.113.10"},
			trustedProxies: trusted,
			want:           "203.0.113.10",
		},
		{
			name:           "複数のプロキシを経由したら信頼しない最も近いアドレスを使う",
			remoteAddr:     "172.18.0.1:54321",
			forwardedFor:   []string{"203.0.113.10, 172.18.0.9"},
			trustedProxies: trusted,
			want:           "203.0.113.10",
		},
		{
			name:           "ヘッダーが複数に分かれていても1本のチェーンとして読む",
			remoteAddr:     "172.18.0.1:54321",
			forwardedFor:   []string{"203.0.113.10", "172.18.0.9"},
			trustedProxies: trusted,
			want:           "203.0.113.10",
		},
		{
			name:       "左端のクライアントが名乗った値は採らない",
			remoteAddr: "172.18.0.1:54321",
			// 左端はクライアントが名乗った値。これを採ると誰でも他人のアドレスを装える。
			forwardedFor:   []string{"198.51.100.200, 203.0.113.10"},
			trustedProxies: trusted,
			want:           "203.0.113.10",
		},
		{
			name:       "クライアントが書いた信頼するネットワークのアドレスをプロキシとして扱わない",
			remoteAddr: "172.18.0.1:54321",
			// 信頼しない最初のアドレスで止まるため、その外側にプロキシを装った値があっても辿らない。
			forwardedFor:   []string{"172.18.0.200, 203.0.113.10"},
			trustedProxies: trusted,
			want:           "203.0.113.10",
		},
		{
			name:           "読めない項目に当たったら接続元へ戻す",
			remoteAddr:     "172.18.0.1:54321",
			forwardedFor:   []string{"unknown, 172.18.0.9"},
			trustedProxies: trusted,
			want:           "172.18.0.1",
		},
		{
			name:           "チェーンが信頼するプロキシだけなら接続元へ戻す",
			remoteAddr:     "172.18.0.1:54321",
			forwardedFor:   []string{"172.18.0.9"},
			trustedProxies: trusted,
			want:           "172.18.0.1",
		},
		{
			name:           "転送ヘッダーが無ければ接続元を使う",
			remoteAddr:     "172.18.0.1:54321",
			trustedProxies: trusted,
			want:           "172.18.0.1",
		},
		{
			name:           "ポート付きの項目も読む",
			remoteAddr:     "172.18.0.1:54321",
			forwardedFor:   []string{"203.0.113.10:443"},
			trustedProxies: trusted,
			want:           "203.0.113.10",
		},
		{
			name:           "ブラケットで囲まれたIPv6も読む",
			remoteAddr:     "[2001:db8::1]:54321",
			forwardedFor:   []string{"[2001:db8:ffff::1]"},
			trustedProxies: prefixes(t, "2001:db8::/48"),
			want:           "2001:db8:ffff::1",
		},
		{
			name:           "IPv6に埋め込まれたIPv4はIPv4の範囲と照合する",
			remoteAddr:     "[::ffff:172.18.0.1]:54321",
			forwardedFor:   []string{"203.0.113.10"},
			trustedProxies: trusted,
			want:           "203.0.113.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for _, value := range tt.forwardedFor {
				req.Header.Add("X-Forwarded-For", value)
			}

			if got := clientip.Resolve(req, tt.trustedProxies); got != tt.want {
				t.Errorf("Resolve() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestResolve_UnparsableRemoteAddr は、接続元を解析できないときに値をそのまま返すことを検証する。
// 記録する値が空になるより、解析できなかった値を残すほうが調査の手がかりになる。
func TestResolve_UnparsableRemoteAddr(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "pipe"

	if got := clientip.Resolve(req, nil); got != "pipe" {
		t.Errorf("Resolve() = %q、期待値 = %q", got, "pipe")
	}
}
