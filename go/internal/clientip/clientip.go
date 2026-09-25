// Package clientip はHTTPリクエストからクライアントのIPアドレスを解決する。
package clientip

import (
	"net/http"
	"net/netip"
	"strings"
)

// forwardedForHeader は解決が読む唯一の転送ヘッダー。
//
// プロキシはこのヘッダーを置き換えるのではなく追記するため、下のチェーンの走査で
// 経由したプロキシとクライアントが書いた値を見分けられる。
// プロキシが素通しするだけのヘッダー (CF-Connecting-IP・X-Real-IP) にはその根拠が無く、
// 上書きしないプロキシの背後にいる者が任意のアドレスを書ける。
const forwardedForHeader = "X-Forwarded-For"

// Resolve はリクエストを送ってきたクライアントのIPアドレスを返す。
//
// trustedProxies には、このアプリの前段に立つリバースプロキシが接続してくるネットワークを渡す。
// 1つも渡されなければ接続元のアドレスをそのまま使い、転送ヘッダーは読まない。
// 追記すると分かっているプロキシが前段に立つまで、転送ヘッダーはクライアントが書いた入力に過ぎないためである。
//
// 接続元が信頼するプロキシのときは、X-Forwarded-For のチェーンを近い側から外へ辿り、
// 信頼するプロキシではない最初のアドレスをクライアントとして採る。
// 近い側から辿るのは、各プロキシが自分の見た接続元を末尾に追記するため。
// 左端の項目はクライアントが書いた値そのもので、それを採ると誰でも自分のアドレスを名乗れる。
// アドレスとして読めない項目 ("unknown" など) に当たったらそこで止め、接続元のアドレスに戻す。
// 位置づけられない項目はプロキシであることも示せず、越えて辿るとクライアントが操作できる値に届く。
//
// 前段が複数あるときは、最も近い1つだけでなくすべてを trustedProxies に入れる。
// 入れ忘れたプロキシは「信頼しないアドレス」として採られ、訪問者がそのプロキシのものになる。
func Resolve(r *http.Request, trustedProxies []netip.Prefix) string {
	peer, ok := parseAddr(r.RemoteAddr)
	if !ok {
		return r.RemoteAddr
	}

	if !isTrusted(peer, trustedProxies) {
		return peer.String()
	}

	if client, ok := forwardedClient(r.Header.Values(forwardedForHeader), trustedProxies); ok {
		return client.String()
	}

	return peer.String()
}

// forwardedClient は X-Forwarded-For のチェーンを近い側から外へ読んだとき、
// 信頼するプロキシではない最初のアドレスを返す。
//
// ヘッダーはチェーン全体を持つ1つの値で届くことも、経由したプロキシごとに1つずつ届くこともある。
// どちらで届くかは経由したプロキシ次第で、値をまたいだ並びは追記された順そのものであるため、
// 値の並びをカンマ区切りの1本のチェーンとして読む。
func forwardedClient(values []string, trustedProxies []netip.Prefix) (netip.Addr, bool) {
	entries := strings.Split(strings.Join(values, ","), ",")

	for i := len(entries) - 1; i >= 0; i-- {
		addr, ok := parseAddr(entries[i])
		if !ok {
			return netip.Addr{}, false
		}

		if !isTrusted(addr, trustedProxies) {
			return addr, true
		}
	}

	return netip.Addr{}, false
}

// parseAddr は RemoteAddr や転送ヘッダーに現れる形のアドレスを1つ解析する。
//
// ポート付きも受け付けるのは、RemoteAddrが常にポートを伴い、転送ヘッダーも伴うことがあるため。
// IPv6に埋め込まれたIPv4アドレスは展開し、運用者がそのアドレスに対して書くIPv4の範囲と照合できるようにする。
// IPv6のゾーンは、アドレスの範囲がインターフェイスに依存しないため取り除く。
func parseAddr(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)

	if addr, err := netip.ParseAddr(value); err == nil {
		return normalizeAddr(addr), true
	}
	if addrPort, err := netip.ParseAddrPort(value); err == nil {
		return normalizeAddr(addrPort.Addr()), true
	}
	if host, ok := unbracketedHost(value); ok {
		if addr, err := netip.ParseAddr(host); err == nil {
			return normalizeAddr(addr), true
		}
	}

	return netip.Addr{}, false
}

// unbracketedHost は "[2001:db8::1]" のようにブラケットで囲まれたIPv6アドレスから、その囲みを取り除く。
//
// ポートを追記するときにアドレスをブラケットで囲むプロキシは、ポートを省くときも囲みを残す。
// この形を読めないとチェーンの走査がそこで止まり、訪問者が前段のプロキシのものとして扱われる。
// 両側の囲みを求めるのは、途中で切れた項目をプロキシが書いたものとして採らないためである。
func unbracketedHost(value string) (string, bool) {
	host, ok := strings.CutPrefix(value, "[")
	if !ok {
		return "", false
	}

	return strings.CutSuffix(host, "]")
}

// normalizeAddr はアドレスを範囲の照合に使う形へ揃える。
func normalizeAddr(addr netip.Addr) netip.Addr {
	return addr.Unmap().WithZone("")
}

// isTrusted はaddrが信頼するプロキシのネットワークに属するかどうかを返す。
func isTrusted(addr netip.Addr, trustedProxies []netip.Prefix) bool {
	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
