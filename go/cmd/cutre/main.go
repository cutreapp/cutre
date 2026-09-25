// cutreコマンドはCutreのコマンドラインのエントリーポイント。
// serveサブコマンドがHTTPサーバーを起動し、seedサブコマンドが開発用のアカウントを作る。
// invitationサブコマンドは、管理者が招待者の無い招待リンクを発行するのに使う。
// userサブコマンドは、管理者がユーザーの二要素認証を無効にするのに使う。
package main

import (
	"fmt"
	"io"
	"os"
)

// exitUsage はサブコマンドが指定されていない / 未知の場合に使う終了コード。
// 使用方法の誤りを2、依頼された処理自体の失敗を1とするGoツールチェインやgetoptの慣習に従う。
const exitUsage = 2

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run はargsをサブコマンドへ振り分け、プロセスの終了コードを返す。
// os.Args を直接読んでプロセスのストリームへ書かず引数で受け取るのは、
// テストプロセスを終了させずに振り分けをテストできるようにするため。
//
// サブコマンド無しのときにserveへ既定することはしない。
// 何も指定しない実行はusageを表示して失敗するため、各呼び出し箇所がどのサブコマンドを使うのかを明示することになる。
func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)

		return exitUsage
	}

	switch args[0] {
	case "serve":
		return runServe()
	case "seed":
		return runSeed(os.Stdout)
	case "invitation":
		return runInvitation(args[1:], os.Stdout, stderr)
	case "user":
		return runUser(args[1:], os.Stdout, stderr)
	default:
		// ここは診断情報の出力先そのものであり、書き込みの失敗を報告する先が残っていないためエラーを捨てる。
		// 何が起きたかは終了コードで呼び出し側に伝わる。
		_, _ = fmt.Fprintf(stderr, "未知のサブコマンドです: %q\n\n", args[0])
		usage(stderr)

		return exitUsage
	}
}

// usage は利用可能なサブコマンドの一覧をwに書く。
// 書き込みエラーを捨てる理由はrunと同じ。
func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `使い方: cutre <コマンド>

コマンド:
  serve      HTTPサーバーを起動する
  seed       開発用のアカウントを作る (開発環境のみ)
  invitation 招待リンクを発行する
  user       ユーザーの二要素認証を無効にする
`)
}
