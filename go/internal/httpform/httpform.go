// Package httpform はハンドラーが共有する、フォームの値の読み取りを提供する。
//
// 管理画面とユーザー向けの画面のどちらも、編集の画面を開いたときの版をフォームで持ち回り、送信時に競合を見つける。
// 版が読めない送信の扱いを画面ごとに書き分けず、ここで1つに持つ。
package httpform

import (
	"net/http"
	"strconv"
)

// lockVersionFieldName はフォームが持ち回る版 (lock_version) のフィールドの名前。
const lockVersionFieldName = "lock_version"

// MissingLockVersion はフォームに版が無い、または読めないときに LockVersion が返す値。
// 版は0から上がっていくため、この値と一致する行は無く、更新は必ず競合になる。
const MissingLockVersion int32 = -1

// LockVersion はフォームが持ち回った版を読む。
//
// 版が無い、または読めない送信は、どの版から操作したかが分からないため MissingLockVersion を返し、
// 古い版からの送信と同じく競合として扱わせる。
// DELETEの本文はGoがフォームとして読まないため、POSTに _method を載せて送り、MethodOverride が読んだ値を使う。
func LockVersion(r *http.Request) int32 {
	version, err := strconv.ParseInt(r.PostFormValue(lockVersionFieldName), 10, 32)
	if err != nil {
		return MissingLockVersion
	}

	return int32(version)
}
