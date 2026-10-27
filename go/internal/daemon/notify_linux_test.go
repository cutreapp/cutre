package daemon

import (
	"crypto/rand"
	"net"
	"testing"
)

// TestNotify_AbstractSocket は、systemdが渡すことのある抽象名前空間のアドレス (先頭が `@`) へ送れることを検証する。
func TestNotify_AbstractSocket(t *testing.T) {
	t.Parallel()

	socket := "@cutre-notify-test-" + rand.Text()
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socket, Net: "unixgram"})
	if err != nil {
		t.Fatalf("ListenUnixgram()のエラー = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := notify(socket, "READY=1"); err != nil {
		t.Fatalf("notify()のエラー = %v", err)
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Read()のエラー = %v", err)
	}
	if got := string(buf[:n]); got != "READY=1" {
		t.Errorf("受け取った状態 = %q、期待値 = %q", got, "READY=1")
	}
}
