package daemon

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
)

// listenNotifySocket はsystemdの代わりに通知を受けるソケットを作り、そのアドレスを返す。
func listenNotifySocket(t *testing.T) (*net.UnixConn, string) {
	t.Helper()

	socket := filepath.Join(t.TempDir(), "notify.sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socket, Net: "unixgram"})
	if err != nil {
		t.Fatalf("ListenUnixgram()のエラー = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return conn, socket
}

func TestNotifyReady(t *testing.T) {
	// t.Setenv を使うため t.Parallel() を呼ばない。
	conn, socket := listenNotifySocket(t)
	t.Setenv("NOTIFY_SOCKET", socket)

	if err := NotifyReady(); err != nil {
		t.Fatalf("NotifyReady()のエラー = %v", err)
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

func TestNotifyReady_NoSocket(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "")

	if err := NotifyReady(); err != nil {
		t.Errorf("NotifyReady()のエラー = %v、nilを期待", err)
	}
}

func TestNotify_Unreachable(t *testing.T) {
	t.Parallel()

	socket := filepath.Join(t.TempDir(), "missing.sock")

	err := notify(socket, "READY=1")
	if err == nil {
		t.Fatal("エラーを期待したが、nilだった")
	}
	if !strings.Contains(err.Error(), socket) {
		t.Errorf("エラーメッセージ = %q、ソケットのパスを含むことを期待", err.Error())
	}
}
