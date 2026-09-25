package usecase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateAccountUsecase は、確認を済ませたメールアドレスと入力されたアットネーム・パスワードでアカウントを作る。
//
// メールアドレスはフォームではなく確認済みの確認から取る。確認が、本人がそのアドレスを持っていることの証明になるため。
// ユーザーとパスワードの作成、招待の使用の記録、確認の削除を1つのトランザクションで行う。
// ログイン (セッションの発行) は、ハンドラーが CreateSessionUsecase で続けて行う。
type CreateAccountUsecase struct {
	db                       *sql.DB
	invitationRepo           *repository.InvitationRepository
	invitationRedemptionRepo *repository.InvitationRedemptionRepository
	emailConfirmationRepo    *repository.EmailConfirmationRepository
	accountValidator         *validator.AccountCreateValidator
	userRepo                 *repository.UserRepository
	userPasswordRepo         *repository.UserPasswordRepository
}

// NewCreateAccountUsecase は CreateAccountUsecase を生成する。
func NewCreateAccountUsecase(
	db *sql.DB,
	invitationRepo *repository.InvitationRepository,
	invitationRedemptionRepo *repository.InvitationRedemptionRepository,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	accountValidator *validator.AccountCreateValidator,
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
) *CreateAccountUsecase {
	return &CreateAccountUsecase{
		db:                       db,
		invitationRepo:           invitationRepo,
		invitationRedemptionRepo: invitationRedemptionRepo,
		emailConfirmationRepo:    emailConfirmationRepo,
		accountValidator:         accountValidator,
		userRepo:                 userRepo,
		userPasswordRepo:         userPasswordRepo,
	}
}

// CreateAccountInput は CreateAccountUsecase.Execute の入力。
type CreateAccountInput struct {
	// InvitationID と EmailConfirmationID はCookieが運ぶ、受け取った招待と確認を済ませた確認のID。
	InvitationID        model.InvitationID
	EmailConfirmationID model.EmailConfirmationID
	Atname              string
	Password            string
	// Locale はアカウントの表示言語。登録の画面の言語版で決まる。
	Locale model.Locale
}

// CreateAccountOutput は CreateAccountUsecase.Execute の結果。
type CreateAccountOutput struct {
	User *model.User
}

// Execute は確認と招待を確かめ、入力を検証してからアカウントを作る。
//
// 確認済みの確認が無いときは AppErrCodeResourceNotFound を、招待が登録に使えないときは AppErrCodeForbidden を返す。
// どちらもフォームを直して解決できる失敗ではないため、*model.ValidationError とは分ける。
func (uc *CreateAccountUsecase) Execute(ctx context.Context, input CreateAccountInput) (*CreateAccountOutput, error) {
	confirmation, err := uc.emailConfirmationRepo.FindConfirmedByID(ctx, input.EmailConfirmationID)
	if err != nil {
		return nil, fmt.Errorf("確認済みのメールアドレスの確認の取得に失敗: %w", err)
	}
	if confirmation == nil {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	invitation, err := uc.invitationRepo.FindByID(ctx, input.InvitationID)
	if err != nil {
		return nil, fmt.Errorf("招待の取得に失敗: %w", err)
	}
	usable, err := isUsableInvitation(ctx, uc.invitationRedemptionRepo, invitation)
	if err != nil {
		return nil, err
	}
	if !usable {
		return nil, invitationUnusableError(ctx)
	}

	if err := uc.accountValidator.Validate(ctx, validator.AccountCreateValidatorInput{
		Email:    confirmation.Email,
		Atname:   input.Atname,
		Password: input.Password,
	}); err != nil {
		return nil, err
	}

	passwordDigest, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("パスワードのハッシュ化に失敗: %w", err)
	}

	return uc.createAccount(ctx, input, confirmation.Email, passwordDigest)
}

// createAccount はユーザーとパスワードを作り、招待の使用を記録し、確認を消すまでを1つのトランザクションで行う。
//
// 招待は行を排他ロックしてから、人数を数え直して使えることを確かめる。
// 取り消していない招待は招待者ごとに1本に限られ、使える招待は取り消していないものだけのため、
// 同じ招待者の招待で同時に登録しても、同じ行のロックを待ち合わせることになり、招待者単位に直列化される。
// 確認の削除は確認済みの行だけを対象にする条件付きの削除にし、同じ確認で先に登録されたときは、アカウントを作らずに失敗させる。
func (uc *CreateAccountUsecase) createAccount(ctx context.Context, input CreateAccountInput, email, passwordDigest string) (*CreateAccountOutput, error) {
	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	invitation, err := uc.invitationRepo.WithTx(tx).FindByIDForUpdate(ctx, input.InvitationID)
	if err != nil {
		return nil, fmt.Errorf("招待の取得に失敗: %w", err)
	}
	usable, err := isUsableInvitation(ctx, uc.invitationRedemptionRepo.WithTx(tx), invitation)
	if err != nil {
		return nil, err
	}
	if !usable {
		return nil, invitationUnusableError(ctx)
	}

	user, err := uc.userRepo.WithTx(tx).Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   input.Atname,
		Locale:   input.Locale,
		TimeZone: model.DefaultTimeZone,
	})
	if err != nil {
		if ve := takenValueError(ctx, err); ve != nil {
			return nil, ve
		}
		return nil, fmt.Errorf("ユーザーの作成に失敗: %w", err)
	}

	if _, err := uc.userPasswordRepo.WithTx(tx).Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: passwordDigest,
	}); err != nil {
		return nil, fmt.Errorf("パスワードの作成に失敗: %w", err)
	}

	if _, err := uc.invitationRedemptionRepo.WithTx(tx).Create(ctx, invitation.ID, user.ID); err != nil {
		return nil, fmt.Errorf("招待の使用の記録に失敗: %w", err)
	}

	deleted, err := uc.emailConfirmationRepo.WithTx(tx).DeleteConfirmed(ctx, input.EmailConfirmationID)
	if err != nil {
		return nil, fmt.Errorf("メールアドレスの確認の削除に失敗: %w", err)
	}
	if !deleted {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreateAccountOutput{User: user}, nil
}

// invitationUnusableError は、招待が登録に使えないためにアカウントを作れないことを表すエラーを返す。
func invitationUnusableError(ctx context.Context) *model.AppError {
	return &model.AppError{
		Code:    model.AppErrCodeForbidden,
		UserMsg: i18n.T(ctx, "account_new_invitation_unusable_message"),
	}
}

// takenValueError は、ユーザーの作成が一意制約で拒まれたときに、Validatorと同じメッセージのエラーを返す。
// 空きを確かめてから作成するまでの間に、同じ値で先に登録された場合に当たる。それ以外のエラーにはnilを返す。
func takenValueError(ctx context.Context, err error) *model.ValidationError {
	ve := model.NewValidationError()
	switch {
	case errors.Is(err, repository.ErrUserAtnameTaken):
		ve.AddField("atname", i18n.T(ctx, "validation_atname_taken"))
	case errors.Is(err, repository.ErrUserEmailTaken):
		ve.AddGlobal(i18n.T(ctx, "validation_account_email_taken"))
	default:
		return nil
	}

	return ve
}
