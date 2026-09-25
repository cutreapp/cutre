package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// DeleteSessionUsecase はセッションを削除してログアウトさせる。
//
// セッションCookieの消去はハンドラーの別の段階とする。
// 行を消しておけば、Cookieを消す前に盗まれたトークンが後から使われても、ユーザーに解決しない。
// 入力はリクエスト自身のセッショントークンで利用者のフォームの入力ではないため、バリデーターを持たない。
type DeleteSessionUsecase struct {
	userSessionRepo *repository.UserSessionRepository
}

// NewDeleteSessionUsecase は DeleteSessionUsecase を生成する。
func NewDeleteSessionUsecase(userSessionRepo *repository.UserSessionRepository) *DeleteSessionUsecase {
	return &DeleteSessionUsecase{userSessionRepo: userSessionRepo}
}

// DeleteSessionInput は DeleteSessionUsecase.Execute の入力。
type DeleteSessionInput struct {
	// Token はリクエストのセッションCookieの平文のトークン。未ログインのときは空文字列。
	Token string
}

// Execute はinput.Tokenが指すセッションを削除する。
//
// トークンが空のとき (未ログインでのログアウト) は何もしない。
// 既に消えたセッションや未知のトークンでも、削除する行が無いだけでエラーにはならない。
// ログアウトは何度行っても同じ結果に落ち着くべき操作のため。
func (uc *DeleteSessionUsecase) Execute(ctx context.Context, input DeleteSessionInput) error {
	if input.Token == "" {
		return nil
	}

	if err := uc.userSessionRepo.DeleteByTokenDigest(ctx, auth.HashToken(input.Token)); err != nil {
		return fmt.Errorf("セッションの削除に失敗: %w", err)
	}

	return nil
}
