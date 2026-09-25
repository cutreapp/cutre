package model

import "time"

// InvitationLifetime は管理者がCLIから発行する招待リンクの有効期間。
const InvitationLifetime = 7 * 24 * time.Hour

// userInvitationLifetimeDays は、ユーザーの招待リンクを発行日から何日後の日の終わりまで使えるようにするか。
const userInvitationLifetimeDays = 7

// InviterRedemptionLimit は、1人の招待者の招待で登録できる累計の人数。
// 招待で登録した人が退会しても使用の記録は残り、人数は戻らない。
const InviterRedemptionLimit = 5

// adminInvitationRedemptionLimit は、管理者がCLIから発行した招待1つで登録できる人数。
const adminInvitationRedemptionLimit = 1

// InvitationExpiresAt はnowに発行した招待の有効期限を返す。
func InvitationExpiresAt(now time.Time) time.Time {
	return now.Add(InvitationLifetime)
}

// UserInvitationExpiresAt は、ユーザーがnowに発行した招待の有効期限を返す。
//
// 招待者のタイムゾーン loc で、発行日から7日後の日の終わり (その翌日の0時) にする。
// 画面には期限を日付だけで示すため、示した日付の終わりまで実際に使えるよう期限そのものを日の境目に揃える。
func UserInvitationExpiresAt(now time.Time, loc *time.Location) time.Time {
	year, month, day := now.In(loc).Date()
	return time.Date(year, month, day+userInvitationLifetimeDays+1, 0, 0, 0, 0, loc)
}

// InviterRemainingRedemptions は、招待者の招待でこれから登録できる人数を返す。
// redeemedCount は招待者のすべての招待の使用の件数。
func InviterRemainingRedemptions(redeemedCount int) int {
	return max(InviterRedemptionLimit-redeemedCount, 0)
}

// Invitation は招待リンク。人数の上限の範囲で、1つの招待を複数人が使える。
//
// InviterUserID は招待したユーザーで、nilは管理者がCLIから発行した招待を表す。
// Token は招待リンク (/i/{token}) に載せる値で、URLとQRコードを再表示できるよう平文で持つ。
// RevokedAt は招待を取り消した時刻で、取り消していなければnil。
// 招待の使用は InvitationRedemption に記録する。
type Invitation struct {
	ID            InvitationID
	InviterUserID *UserID
	Token         string
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// IsAdminIssued は管理者がCLIから発行した (招待者の無い) 招待かを返す。
func (i *Invitation) IsAdminIssued() bool {
	return i.InviterUserID == nil
}

// RedemptionLimit は招待で登録できる人数の上限を返す。
//
// 管理者の招待はその招待1つで数え、ユーザーの招待は招待者のすべての招待を合わせて数える。
// ユーザーの招待を作り直しても人数が戻らないようにするため。
func (i *Invitation) RedemptionLimit() int {
	if i.IsAdminIssued() {
		return adminInvitationRedemptionLimit
	}

	return InviterRedemptionLimit
}

// IsUsable はnowの時点で招待を登録に使えるかを返す。
// 未取り消しで、有効期限が過ぎておらず、人数に空きがあるものだけを使える。
//
// redeemedCount は RedemptionLimit と同じ数え方の、これまでの使用の件数
// (管理者の招待ではその招待の件数、ユーザーの招待では招待者のすべての招待の件数)。
func (i *Invitation) IsUsable(now time.Time, redeemedCount int) bool {
	return i.RevokedAt == nil && now.Before(i.ExpiresAt) && redeemedCount < i.RedemptionLimit()
}

// LastUsableDate は、locのタイムゾーンで招待を使える最後の日の時刻を返す。
// 期限は日の境目に揃えているため、その直前の時刻の日付が「この日まで使える」日になる。
func (i *Invitation) LastUsableDate(loc *time.Location) time.Time {
	return i.ExpiresAt.Add(-time.Nanosecond).In(loc)
}
