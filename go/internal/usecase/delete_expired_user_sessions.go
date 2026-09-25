package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
)

// DeleteExpiredUserSessionsUsecase は有効期限が切れたセッションを削除する。
//
// 期限切れのセッションは現在のユーザーの解決で既に除外されており、残しても使われない。
// 定期ジョブから呼ばれ、利用者の入力を受け取らないため、バリデーターを持たない。
type DeleteExpiredUserSessionsUsecase struct {
	userSessionRepo *repository.UserSessionRepository
}

// NewDeleteExpiredUserSessionsUsecase は DeleteExpiredUserSessionsUsecase を生成する。
func NewDeleteExpiredUserSessionsUsecase(userSessionRepo *repository.UserSessionRepository) *DeleteExpiredUserSessionsUsecase {
	return &DeleteExpiredUserSessionsUsecase{userSessionRepo: userSessionRepo}
}

// Execute は現在時刻までに有効期限が切れたセッションを削除する。
func (uc *DeleteExpiredUserSessionsUsecase) Execute(ctx context.Context) error {
	if err := uc.userSessionRepo.DeleteExpired(ctx, time.Now()); err != nil {
		return fmt.Errorf("期限切れのセッションの削除に失敗: %w", err)
	}

	return nil
}
