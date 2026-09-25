package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// PasswordResetSender はパスワードリセットのリンクを載せたメールを送る。
// UseCaseがテンプレートに依存しないよう、リンクとメールの組み立ては実装 (email.PasswordResetSender) に任せる。
type PasswordResetSender interface {
	Send(ctx context.Context, to, token string, locale model.Locale) error
}

// SendPasswordResetUsecase はリンクのトークンを発行し、パスワードリセットのメールを送る。ジョブのワーカーから呼ばれる。
//
// トークンをジョブを投入するときではなくここで作るのは、平文のトークンをジョブの引数としてデータベースに残さないため。
// 平文はメールに載せるだけで、保存するのはダイジェストだけになる。
type SendPasswordResetUsecase struct {
	db                     *sql.DB
	userRepo               *repository.UserRepository
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
	sender                 PasswordResetSender
}

// NewSendPasswordResetUsecase は SendPasswordResetUsecase を生成する。
func NewSendPasswordResetUsecase(
	db *sql.DB,
	userRepo *repository.UserRepository,
	passwordResetTokenRepo *repository.PasswordResetTokenRepository,
	sender PasswordResetSender,
) *SendPasswordResetUsecase {
	return &SendPasswordResetUsecase{
		db:                     db,
		userRepo:               userRepo,
		passwordResetTokenRepo: passwordResetTokenRepo,
		sender:                 sender,
	}
}

// SendPasswordResetInput は SendPasswordResetUsecase.Execute の入力。
type SendPasswordResetInput struct {
	UserID model.UserID
	Locale model.Locale
}

// Execute はユーザーのトークンを新しいものに置き換え、そのトークンのリンクをメールで送る。
// 失敗はそのまま返し、ジョブの再試行に任せる。
//
// 申請からジョブの処理までの間に退会したユーザーには送らない。
//
// 送信にIdempotencyKeyを添えない。再試行のたびにトークンを作り直すため、Resendが前の試行の送信とまとめると、
// 置き換えて使えなくなったトークンのメールだけが届くことになる。
// 前の試行の送信が実は届いていた場合はメールが2通になるが、新しいほうのリンクが使える。
func (uc *SendPasswordResetUsecase) Execute(ctx context.Context, input SendPasswordResetInput) error {
	token, err := auth.GenerateSecureToken()
	if err != nil {
		return fmt.Errorf("パスワードリセットのトークンの生成に失敗: %w", err)
	}

	user, err := uc.replaceToken(ctx, input.UserID, auth.HashToken(token), model.PasswordResetTokenExpiresAt(time.Now()))
	if err != nil {
		return err
	}
	if user == nil {
		return nil
	}

	if err := uc.sender.Send(ctx, user.Email, token, input.Locale); err != nil {
		return fmt.Errorf("パスワードリセットのメールの送信に失敗: %w", err)
	}

	return nil
}

// replaceToken はユーザーの古いトークンを消して新しいトークンを作る処理を、1つのトランザクションで行う。
// 以前に発行したリンクを使えなくし、ユーザーが使えるリンクを最後に保存した1つだけにする。
//
// ユーザーはロックを取ると同時に読み、退会済みならトークンを作らずnilを返す。
// ロックの外で読むと、読んでからロックを取るまでに確定した退会を見落とし、匿名化する前のアドレスへ送ってしまう。
func (uc *SendPasswordResetUsecase) replaceToken(ctx context.Context, userID model.UserID, tokenDigest string, expiresAt time.Time) (*model.User, error) {
	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// トークンがまだ無い場合も同じユーザーの置換を直列化する。
	user, err := uc.userRepo.WithTx(tx).LockByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("パスワードリセットの対象ユーザーのロックに失敗: %w", err)
	}
	if user == nil {
		return nil, nil
	}

	tokenRepo := uc.passwordResetTokenRepo.WithTx(tx)
	if err := tokenRepo.DeleteByUserID(ctx, userID); err != nil {
		return nil, fmt.Errorf("古いパスワードリセットのトークンの削除に失敗: %w", err)
	}
	if _, err := tokenRepo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: tokenDigest,
		ExpiresAt:   expiresAt,
	}); err != nil {
		return nil, fmt.Errorf("パスワードリセットのトークンの保存に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return user, nil
}
