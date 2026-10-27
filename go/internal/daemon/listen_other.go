//go:build !linux

package daemon

import "syscall"

// reusePort はLinux以外では何もしない。
// SO_REUSEPORT の意味がOSごとに異なり (BSD系では負荷分散しない)、本番はLinuxだけで動かすため。
func reusePort(_, _ string, _ syscall.RawConn) error {
	return nil
}
