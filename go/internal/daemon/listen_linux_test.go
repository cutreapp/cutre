package daemon

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// TestListen_ReusePort は、デプロイで新旧のプロセスが並ぶときのように、同じポートで2つ目の待ち受けを始められることを検証する。
func TestListen_ReusePort(t *testing.T) {
	t.Parallel()

	first, err := Listen(context.Background(), "127.0.0.1:0")
	if err != nil {
		t.Fatalf("1つ目のListen()のエラー = %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	second, err := Listen(context.Background(), first.Addr().String())
	if err != nil {
		t.Fatalf("同じアドレスでの2つ目のListen()のエラー = %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
}

// TestListen_MigratePendingConnection は、旧リスナーのaccept待ちの接続を新リスナーが引き継げることを検証する。
// カーネル設定が必要なため、有効にできない開発環境ではスキップし、CIで必ず有効にして実行する。
func TestListen_MigratePendingConnection(t *testing.T) {
	t.Parallel()

	value, err := os.ReadFile("/proc/sys/net/ipv4/tcp_migrate_req")
	if err != nil {
		t.Fatalf("tcp_migrate_reqの読み取りに失敗しました: %v", err)
	}
	if strings.TrimSpace(string(value)) != "1" {
		t.Skip("未accept接続の移行には net.ipv4.tcp_migrate_req=1 が必要です")
	}

	first, err := Listen(context.Background(), "127.0.0.1:0")
	if err != nil {
		t.Fatalf("旧リスナーのListen()のエラー = %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	// 新リスナーを作る前に接続し、旧リスナーのacceptキューに確実に入れる。
	conn, err := net.DialTimeout("tcp", first.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatalf("DialTimeout()のエラー = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("クライアントのSetDeadline()のエラー = %v", err)
	}

	const request = "GET /health HTTP/1.0\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatalf("リクエストの送信に失敗しました: %v", err)
	}

	second, err := Listen(context.Background(), first.Addr().String())
	if err != nil {
		t.Fatalf("新リスナーのListen()のエラー = %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if err := second.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("新リスナーのSetDeadline()のエラー = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("旧リスナーのClose()のエラー = %v", err)
	}

	accepted, err := second.Accept()
	if err != nil {
		t.Fatalf("新リスナーが接続を引き継げませんでした: %v", err)
	}
	t.Cleanup(func() { _ = accepted.Close() })
	if err := accepted.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("引き継いだ接続のSetDeadline()のエラー = %v", err)
	}

	buf := make([]byte, len(request))
	if _, err := io.ReadFull(accepted, buf); err != nil {
		t.Fatalf("引き継いだリクエストの読み取りに失敗しました: %v", err)
	}
	if got := string(buf); got != request {
		t.Errorf("引き継いだリクエスト = %q、期待値 = %q", got, request)
	}

	const response = "HTTP/1.0 200 OK\r\nContent-Length: 2\r\n\r\nok"
	if _, err := io.WriteString(accepted, response); err != nil {
		t.Fatalf("引き継いだ接続への応答に失敗しました: %v", err)
	}
	buf = make([]byte, len(response))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("応答の受信に失敗しました: %v", err)
	}
	if got := string(buf); got != response {
		t.Errorf("受信した応答 = %q、期待値 = %q", got, response)
	}
}
