package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
)

// processToken はテストバイナリの実行ごとに決まる短いランダム文字列。
//
// テストはパッケージごとに別プロセスで同じテスト用データベースを共有し、
// トランザクションで包まないテストが書いた行はコミットされて残る。
// プロセス内の連番だけでは別パッケージの行とUNIQUE制約で衝突しうるため、
// 連番にこのトークンを添えて値をプロセスごとにも分ける。
var processToken = newProcessToken()

// sequence はプロセス内で値を区別するための連番。
var sequence atomic.Int64

func newProcessToken() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("テスト用のランダム値の生成に失敗しました: %v", err))
	}

	return hex.EncodeToString(b)
}

// uniqueToken は呼び出しごとに異なる短い文字列を返す。
func uniqueToken() string {
	return fmt.Sprintf("%s%d", processToken, sequence.Add(1))
}

// UniqueEmail は他と重複しないメールアドレスを返す。
// prefixはそのアドレスが何のためのものかを表し、失敗したテストがどのデータかを示せるようにする。
func UniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%s@example.com", prefix, uniqueToken())
}

// UniqueAtname は他と重複しないアットネームを返す。
// 先頭を英字にするのは、アットネームが許す文字 (半角英数字とアンダースコア) に収めるため。
func UniqueAtname() string {
	return "u" + uniqueToken()
}
