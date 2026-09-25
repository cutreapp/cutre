package model

import "time"

// RateLimit は固定ウィンドウのレート制限のカウンター。
//
// Key は用途・単位の種類・値を連結した文字列 (sign_in:ip:203.0.113.10など) で、
// 用途の異なる試行を別々に数えられるようにする。
// WindowStart はその試行が属する時間枠の開始時刻を表す。
// Count はその枠の中で数えた試行の回数。
type RateLimit struct {
	ID          RateLimitID
	Key         string
	WindowStart time.Time
	Count       int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
