package daemon

import (
	"fmt"
	"net"
	"os"
)

// NotifyReady は、プロセスを監督するsystemdへ起動の完了 (READY=1) を通知する。
//
// systemdは Type=notify のユニットを起動するとき、通知の送り先を環境変数 NOTIFY_SOCKET で渡す。
// 未設定のとき (systemdの外で動かすとき) は何もしない。
func NotifyReady() error {
	return notify(os.Getenv("NOTIFY_SOCKET"), "READY=1")
}

// notify はsocketへ状態を1つ送る。socketが空なら何もしない。
//
// 先頭が `@` のアドレスは抽象名前空間のソケットを指す。
// Linuxでは net パッケージが `@` をNULバイトへ置き換えるため、そのまま渡す。
func notify(socket, state string) error {
	if socket == "" {
		return nil
	}

	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: socket, Net: "unixgram"})
	if err != nil {
		return fmt.Errorf("NOTIFY_SOCKET (%s) へ接続できません: %w", socket, err)
	}
	// データグラムは書き込んだ時点で届いており、閉じる失敗は通知の成否に影響しないため捨てる。
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte(state)); err != nil {
		return fmt.Errorf("NOTIFY_SOCKET (%s) へ %s を送れません: %w", socket, state, err)
	}

	return nil
}
