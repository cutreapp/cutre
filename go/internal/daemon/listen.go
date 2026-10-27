package daemon

import (
	"context"
	"net"
)

// Listen はaddrでTCPの待ち受けを始める。
//
// Linuxでは待ち受けのソケットに SO_REUSEPORT を付け、新旧のプロセスが同じポートを共有できるようにする。
// デプロイで新しいプロセスを起動してから古いプロセスを止めるまでの間、どちらかが接続を受け付け続けるため。
// リスナーを閉じるときに未accept接続も引き継ぐには、同じネットワーク名前空間で
// net.ipv4.tcp_migrate_req=1 を設定する必要がある。SO_REUSEPORTだけでは接続は移行されない。
// Linux以外 (開発者の手元など) では付けず、通常の待ち受けになる。
func Listen(ctx context.Context, addr string) (net.Listener, error) {
	lc := net.ListenConfig{Control: reusePort}

	return lc.Listen(ctx, "tcp", addr)
}
