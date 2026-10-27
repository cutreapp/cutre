// Package daemon は、常駐プロセスとしての待ち受けと、プロセスを監督するsystemdへの起動の通知を扱う。
// 設定もHTTPも知らず、標準ライブラリと golang.org/x/sys だけで完結させる。
package daemon
