package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// CreateSessionUsecase はユーザーのログイン中のセッションを作る。
//
// 入力は前の段階で確定したユーザーIDと、リクエストから得た接続元の情報だけで、
// 利用者のフォームの入力を含まないためバリデーターを持たない。
type CreateSessionUsecase struct {
	userSessionRepo *repository.UserSessionRepository
}

// NewCreateSessionUsecase は CreateSessionUsecase を生成する。
func NewCreateSessionUsecase(userSessionRepo *repository.UserSessionRepository) *CreateSessionUsecase {
	return &CreateSessionUsecase{userSessionRepo: userSessionRepo}
}

// CreateSessionInput は CreateSessionUsecase.Execute の入力。
type CreateSessionInput struct {
	UserID model.UserID
	// IPAddress と UserAgent は、セッションをどこから始めたかの記録として残す。
	IPAddress string
	UserAgent string
}

// CreateSessionOutput は CreateSessionUsecase.Execute の結果。
type CreateSessionOutput struct {
	// Token はセッションCookieに書き込む平文のトークン。
	// データベースにはダイジェストだけを保存するため、平文を得られるのはここだけになる。
	Token string
}

// Execute はセッショントークンを発行し、そのダイジェストでセッションを保存する。
// 書き込みは1回のためトランザクションは開かない。
func (uc *CreateSessionUsecase) Execute(ctx context.Context, input CreateSessionInput) (*CreateSessionOutput, error) {
	token, err := createUserSession(ctx, uc.userSessionRepo, input)
	if err != nil {
		return nil, err
	}

	return &CreateSessionOutput{Token: token}, nil
}

// createUserSession はセッショントークンを発行し、そのダイジェストでセッションを保存して、平文のトークンを返す。
//
// リカバリーコードでのログインのように、ほかの書き込みと同じトランザクションでセッションを作るUseCaseとも共有する。
// トランザクションに参加させるかは、渡すリポジトリ (WithTx の有無) で呼び出し元が決める。
func createUserSession(ctx context.Context, userSessionRepo *repository.UserSessionRepository, input CreateSessionInput) (string, error) {
	token, err := auth.GenerateSecureToken()
	if err != nil {
		return "", fmt.Errorf("セッショントークンの生成に失敗: %w", err)
	}

	if _, err := userSessionRepo.Create(ctx, repository.CreateUserSessionInput{
		UserID:      input.UserID,
		TokenDigest: auth.HashToken(token),
		ExpiresAt:   model.UserSessionExpiresAt(time.Now()),
		IPAddress:   input.IPAddress,
		UserAgent:   input.UserAgent,
	}); err != nil {
		return "", fmt.Errorf("セッションの保存に失敗: %w", err)
	}

	return token, nil
}
