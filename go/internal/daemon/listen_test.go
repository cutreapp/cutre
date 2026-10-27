package daemon

import (
	"context"
	"net"
	"testing"
)

func TestListen(t *testing.T) {
	t.Parallel()

	ln, err := Listen(context.Background(), "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen()のエラー = %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	accepted := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
		accepted <- err
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial()のエラー = %v", err)
	}
	_ = conn.Close()

	if err := <-accepted; err != nil {
		t.Errorf("Accept()のエラー = %v", err)
	}
}
