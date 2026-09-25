package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateSignUpUsecase は入力されたメールアドレスで登録を始め、確認コードを送る。
//
// 登録済みのアドレスでも、画面から見える結果 (確認の行とCookie・確認コードの入力画面) を未登録のときと揃える。
// 違いは送るメールだけで、確認コードの代わりにログインを案内するメールを送る。
// 確認の行は作るがコードは誰にも届かないため、そのまま登録を進めることはできない。
type CreateSignUpUsecase struct {
	db                       *sql.DB
	invitationRepo           *repository.InvitationRepository
	invitationRedemptionRepo *repository.InvitationRedemptionRepository
	signUpValidator          *validator.SignUpCreateValidator
	emailConfirmationRepo    *repository.EmailConfirmationRepository
	dispatcher               *dispatcher.Dispatcher
}

// NewCreateSignUpUsecase は CreateSignUpUsecase を生成する。
func NewCreateSignUpUsecase(
	db *sql.DB,
	invitationRepo *repository.InvitationRepository,
	invitationRedemptionRepo *repository.InvitationRedemptionRepository,
	signUpValidator *validator.SignUpCreateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	dispatcher *dispatcher.Dispatcher,
) *CreateSignUpUsecase {
	return &CreateSignUpUsecase{
		db:                       db,
		invitationRepo:           invitationRepo,
		invitationRedemptionRepo: invitationRedemptionRepo,
		signUpValidator:          signUpValidator,
		emailConfirmationRepo:    emailConfirmationRepo,
		dispatcher:               dispatcher,
	}
}

// CreateSignUpInput は CreateSignUpUsecase.Execute の入力。
type CreateSignUpInput struct {
	InvitationID model.InvitationID
	Email        string
	// Locale はメールを書く言語。登録の画面の言語版で決まる。
	Locale model.Locale
}

// CreateSignUpOutput は CreateSignUpUsecase.Execute の結果。
type CreateSignUpOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Execute はメールアドレスを検証し、確認の行の保存とメールのジョブの投入を行う。
func (uc *CreateSignUpUsecase) Execute(ctx context.Context, input CreateSignUpInput) (*CreateSignUpOutput, error) {
	invitation, err := uc.invitationRepo.FindByID(ctx, input.InvitationID)
	if err != nil {
		return nil, fmt.Errorf("招待の取得に失敗: %w", err)
	}
	if err := checkUsableInvitation(ctx, uc.invitationRedemptionRepo, invitation); err != nil {
		return nil, err
	}

	registeredUser, err := uc.signUpValidator.Validate(ctx, validator.SignUpCreateValidatorInput{Email: input.Email})
	if err != nil {
		return nil, err
	}

	code, err := auth.GenerateConfirmationCode()
	if err != nil {
		return nil, fmt.Errorf("確認コードの生成に失敗: %w", err)
	}

	return uc.createSignUp(ctx, input, code, registeredUser != nil)
}

// createSignUp は確認の行とメールのジョブを1つのトランザクションで作る。
// 片方だけが残ると、コードの届かない確認か、行の無いコードのメールができてしまうため。
func (uc *CreateSignUpUsecase) createSignUp(ctx context.Context, input CreateSignUpInput, code string, registered bool) (*CreateSignUpOutput, error) {
	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 招待の状態をロック付きで読み、確認の保存までの間に取り消されたり、人数の上限まで使われたりしないようにする。
	invitation, err := uc.invitationRepo.WithTx(tx).FindByIDForShare(ctx, input.InvitationID)
	if err != nil {
		return nil, fmt.Errorf("招待の取得に失敗: %w", err)
	}
	if err := checkUsableInvitation(ctx, uc.invitationRedemptionRepo.WithTx(tx), invitation); err != nil {
		return nil, err
	}

	confirmation, err := uc.emailConfirmationRepo.WithTx(tx).Create(ctx, repository.CreateEmailConfirmationInput{
		Email:     input.Email,
		Code:      code,
		ExpiresAt: model.EmailConfirmationExpiresAt(time.Now()),
	})
	if err != nil {
		return nil, fmt.Errorf("メールアドレスの確認の保存に失敗: %w", err)
	}

	jobs := uc.dispatcher.WithTx(tx)
	if registered {
		err = jobs.EnqueueAlreadyRegisteredNotice(ctx, input.Email, input.Locale)
	} else {
		err = jobs.EnqueueEmailConfirmation(ctx, input.Email, code, input.Locale)
	}
	if err != nil {
		return nil, fmt.Errorf("メールを送るジョブの投入に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreateSignUpOutput{EmailConfirmation: confirmation}, nil
}
