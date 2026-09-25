package model

import "time"

// EmailConfirmationLifetime は確認コードの有効期間。
const EmailConfirmationLifetime = 15 * time.Minute

// EmailConfirmationMaxFailedAttempts は1つの確認コードに許す誤入力の回数。
// 6桁のコードを総当たりで当てられないよう、上限に達した確認ではそれ以上コードを照合しない。
const EmailConfirmationMaxFailedAttempts = 5

// EmailConfirmationExpiresAt はnowに送った確認コードの有効期限を返す。
func EmailConfirmationExpiresAt(now time.Time) time.Time {
	return now.Add(EmailConfirmationLifetime)
}

// EmailConfirmation はメールアドレスを持っていることを確かめる1回分の確認コード。
//
// 登録の途中でまだユーザーがいないため、ユーザーではなくメールアドレスに紐づく。
// FailedAttemptsCount は誤ったコードを入力した回数、ConfirmedAt はコードが一致した時刻 (確認前はnil)。
type EmailConfirmation struct {
	ID                  EmailConfirmationID
	Email               string
	Code                string
	ExpiresAt           time.Time
	FailedAttemptsCount int32
	ConfirmedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// HasReachedMaxFailedAttempts は誤入力の回数が上限に達し、もうコードを照合しない状態かを返す。
func (c *EmailConfirmation) HasReachedMaxFailedAttempts() bool {
	return c.FailedAttemptsCount >= EmailConfirmationMaxFailedAttempts
}
