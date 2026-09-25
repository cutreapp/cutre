package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
)

// emailConfirmationRetention は有効期限が切れた確認を残しておく期間。
//
// 確認済みの確認は、確認済みのCookieの寿命 (確認した時点から1時間) のあいだアカウントの作成に使われる。
// 確認は有効期限までに済ませるため、有効期限から1時間を過ぎれば使われることはない。
// レート制限のカウンターと同じく、登録の試行を後から調べられるよう1日残す。
const emailConfirmationRetention = 24 * time.Hour

// DeleteExpiredEmailConfirmationsUsecase は使われずに残ったメールアドレスの確認を削除する。
//
// アカウントの作成に使った確認はその時点で消えるため、残るのは未確認の確認と、確認したがアカウントを作らなかった確認になる。
// どちらもメールアドレスを持つため、保持期間を過ぎたら確認済みかどうかによらず消す。
// 定期ジョブから呼ばれ、利用者の入力を受け取らないため、バリデーターを持たない。
type DeleteExpiredEmailConfirmationsUsecase struct {
	emailConfirmationRepo *repository.EmailConfirmationRepository
}

// NewDeleteExpiredEmailConfirmationsUsecase は DeleteExpiredEmailConfirmationsUsecase を生成する。
func NewDeleteExpiredEmailConfirmationsUsecase(emailConfirmationRepo *repository.EmailConfirmationRepository) *DeleteExpiredEmailConfirmationsUsecase {
	return &DeleteExpiredEmailConfirmationsUsecase{emailConfirmationRepo: emailConfirmationRepo}
}

// Execute は有効期限から保持期間を過ぎた確認を削除する。
func (uc *DeleteExpiredEmailConfirmationsUsecase) Execute(ctx context.Context) error {
	if err := uc.emailConfirmationRepo.DeleteExpired(ctx, time.Now().Add(-emailConfirmationRetention)); err != nil {
		return fmt.Errorf("期限切れのメールアドレスの確認の削除に失敗: %w", err)
	}

	return nil
}
