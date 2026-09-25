package viewmodel

import (
	"context"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// InviterLabel は招待した人の表示名を返す。
//
// 管理者がCLIから発行した招待は招待者を持たないため、運営からの招待として示す。
// 招待者を記録していてもユーザーを引けないのは、その人が退会したときになる。
func InviterLabel(ctx context.Context, invitation *model.Invitation, inviter *model.User) string {
	switch {
	case invitation.InviterUserID == nil:
		return i18n.T(ctx, "invitation_inviter_admin")
	case inviter == nil:
		return i18n.T(ctx, "invitation_inviter_withdrawn")
	default:
		return "@" + inviter.Atname
	}
}

// UserInvitation は招待の画面に出す、ユーザーの使える招待。
type UserInvitation struct {
	// ID は作り直しの二重送信で、画面に表示した招待を識別するために使う。
	ID model.InvitationID
	// URL は招待リンクの絶対URL。コピー・共有・QRコードのいずれもこの値を運ぶ。
	URL string
	// ExpiresOn は招待を使える最後の日。
	ExpiresOn string
}

// NewUserInvitation は招待を画面に出す形にする。
//
// 期限は日の境目に揃えているため、時刻を出さず、使える最後の日の日付だけを招待者のタイムゾーン loc で示す。
func NewUserInvitation(ctx context.Context, invitation *model.Invitation, url string, loc *time.Location, now time.Time) UserInvitation {
	return UserInvitation{
		ID:        invitation.ID,
		URL:       url,
		ExpiresOn: FormatDate(ctx, invitation.LastUsableDate(loc), now.In(loc)),
	}
}

// InvitationRedemption は、招待で参加した人の一覧の1行。
type InvitationRedemption struct {
	// Label は参加した人の表示名 (@アットネーム・退会したユーザー)。
	Label string
	// JoinedOn は参加した日。
	JoinedOn string
}

// NewInvitationRedemptions は招待の使用を、参加した人の一覧の行にする。
//
// 退会した人のアットネームは匿名化した値のため出さず、退会したユーザーとして示す。
// 参加した日は、一覧を見ているユーザーのタイムゾーン loc の日付にする。
func NewInvitationRedemptions(ctx context.Context, redemptions []*model.InvitationRedemption, loc *time.Location, now time.Time) []InvitationRedemption {
	rows := make([]InvitationRedemption, len(redemptions))
	for i, redemption := range redemptions {
		label := i18n.T(ctx, "invitation_redemption_withdrawn")
		if redemption.User != nil && redemption.User.DeletedAt == nil {
			label = "@" + redemption.User.Atname
		}

		rows[i] = InvitationRedemption{
			Label:    label,
			JoinedOn: FormatDate(ctx, redemption.CreatedAt.In(loc), now.In(loc)),
		}
	}

	return rows
}
