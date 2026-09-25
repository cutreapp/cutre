package model

import "time"

// User はアカウントの正準な身元。
// 認証手段 (パスワードなど) は別のモデルが持ち、ここには身元の属性だけを置く。
type User struct {
	ID       UserID
	Email    string
	Atname   string
	Locale   Locale
	TimeZone string

	// DeletedAt は退会した時刻。nilは在籍中を表す。
	// 認証とルックアップのクエリは非nilの行を除外するため、退会したユーザーは
	// ログインの経路に戻らない。
	DeletedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// DefaultTimeZone は登録したアカウントに設定するタイムゾーン。
// タイムゾーンを変える画面を持つまでは、想定する利用者のいる日本の時刻にする。
const DefaultTimeZone = "Asia/Tokyo"

// Location はユーザーのタイムゾーンを返す。日付の表示と、日単位の期限の計算に使う。
func (u *User) Location() (*time.Location, error) {
	return time.LoadLocation(u.TimeZone)
}

// AnonymizedEmail は退会したユーザーのメールアドレスを置き換える値を返す。
//
// ユーザーのIDを含めてユーザーごとに異なる値にし、元のアドレスを次の登録に空ける。
// ドメインは予約されたTLDの .invalid (RFC 2606) で、確認コードが届かないため、この値で登録されることはない。
func AnonymizedEmail(id UserID) string {
	return "deleted-" + id.String() + "@deleted.invalid"
}

// AnonymizedAtname は退会したユーザーのアットネームを置き換える値を返す。
//
// ユーザーのIDを含めてユーザーごとに異なる値にし、元のアットネームを次の登録に空ける。
// アットネームに使えないハイフンを含むため、登録で先に取られて退会が一意制約に負けることはない。
// 画面には出さず (退会したユーザーは「退会したユーザー」と表示する)、アットネームの規則でも確かめないため、
// 最大の長さを超えてもIDの全体を含める。
func AnonymizedAtname(id UserID) string {
	return "deleted-" + id.String()
}
