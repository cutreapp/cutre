package daemon

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// reusePort は待ち受けを始める前のソケットに SO_REUSEPORT を付ける。
func reusePort(_, _ string, c syscall.RawConn) error {
	var sockErr error
	if err := c.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	}); err != nil {
		return err
	}

	return sockErr
}
