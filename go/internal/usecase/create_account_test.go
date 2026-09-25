package usecase_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

const accountPassword = "password1234"

// newCreateAccountUsecase はテスト用のデータベースに直接書き込む CreateAccountUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newCreateAccountUsecase() *usecase.CreateAccountUsecase {
	db := testutil.GetTestDB()
	userRepo := repository.NewUserRepository(db)

	return usecase.NewCreateAccountUsecase(
		db,
		repository.NewInvitationRepository(db),
		repository.NewInvitationRedemptionRepository(db),
		repository.NewEmailConfirmationRepository(db),
		validator.NewAccountCreateValidator(userRepo),
		userRepo,
		repository.NewUserPasswordRepository(db),
	)
}

// confirmedEmail は確認を済ませた確認を作り、そのIDとメールアドレスを返す。
func confirmedEmail(t *testing.T) (model.EmailConfirmationID, string) {
	t.Helper()

	email := testutil.UniqueEmail("account")
	id := testutil.NewEmailConfirmationBuilder(t, testutil.GetTestDB()).WithEmail(email).WithConfirmedAt(time.Now()).Build()

	return id, email
}

// TestCreateAccountUsecase_Execute は、確認のメールアドレスでユーザーとパスワードを作り、
// 招待の使用を記録して確認を消すことを検証する。
func TestCreateAccountUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	invitationID := testutil.NewInvitationBuilder(t, db).Build()
	confirmationID, email := confirmedEmail(t)
	atname := testutil.UniqueAtname()

	output, err := newCreateAccountUsecase().Execute(i18n.SetLocale(ctx, i18n.LangEn), usecase.CreateAccountInput{
		InvitationID:        invitationID,
		EmailConfirmationID: confirmationID,
		Atname:              atname,
		Password:            accountPassword,
		Locale:              model.LocaleEn,
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	user := output.User
	if user.Email != email || user.Atname != atname || user.Locale != model.LocaleEn || user.TimeZone != "Asia/Tokyo" {
		t.Errorf("ユーザー = %+v、メールアドレス %q・アットネーム %q・en・Asia/Tokyo を期待", user, email, atname)
	}

	password, err := repository.NewUserPasswordRepository(db).FindByUserID(ctx, user.ID)
	if err != nil || password == nil {
		t.Fatalf("パスワードの取得 = (%v, %v)、パスワードを期待", password, err)
	}
	if err := auth.CheckPassword(password.PasswordDigest, accountPassword); err != nil {
		t.Errorf("保存したパスワードが入力と一致しない: %v", err)
	}

	var redeemedInvitationID uuid.UUID
	if err := db.QueryRowContext(ctx, "SELECT invitation_id FROM invitation_redemptions WHERE user_id = $1", uuid.UUID(user.ID)).Scan(&redeemedInvitationID); err != nil {
		t.Fatalf("使用の記録の取得のエラー = %v", err)
	}
	if model.InvitationID(redeemedInvitationID) != invitationID {
		t.Errorf("使用の記録の招待 = %v、期待値 = %v", redeemedInvitationID, invitationID)
	}

	if confirmation, err := repository.NewEmailConfirmationRepository(db).FindConfirmedByID(ctx, confirmationID); err != nil || confirmation != nil {
		t.Errorf("確認 = (%+v, %v)、削除を期待", confirmation, err)
	}
}

// TestCreateAccountUsecase_Execute_Rejected は、アカウントを作れない入力・状態で、それぞれのエラーを返し、
// 招待と確認をそのまま残すことを検証する。
func TestCreateAccountUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	db := testutil.GetTestDB()
	uc := newCreateAccountUsecase()

	usedAdminID := testutil.NewInvitationBuilder(t, db).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, usedAdminID).Build()
	fullID := inviterInvitationWithRemaining(t, 0)

	tests := []struct {
		name             string
		invitationID     model.InvitationID
		confirmationID   model.EmailConfirmationID
		atname           string
		wantValidation   bool
		wantCode         model.AppErrorCode
		wantConfirmation bool
	}{
		{
			name:             "形式の誤り",
			invitationID:     testutil.NewInvitationBuilder(t, db).Build(),
			atname:           "cutre-user",
			wantValidation:   true,
			wantConfirmation: true,
		},
		{
			name:             "使用済みの管理者の招待",
			invitationID:     usedAdminID,
			wantCode:         model.AppErrCodeForbidden,
			wantConfirmation: true,
		},
		{
			name:             "招待者の人数の上限に達した招待",
			invitationID:     fullID,
			wantCode:         model.AppErrCodeForbidden,
			wantConfirmation: true,
		},
		{
			name:             "期限切れの招待",
			invitationID:     testutil.NewInvitationBuilder(t, db).WithExpiresAt(time.Now().Add(-time.Minute)).Build(),
			wantCode:         model.AppErrCodeForbidden,
			wantConfirmation: true,
		},
		{
			name:           "未確認の確認",
			invitationID:   testutil.NewInvitationBuilder(t, db).Build(),
			confirmationID: testutil.NewEmailConfirmationBuilder(t, db).Build(),
			wantCode:       model.AppErrCodeResourceNotFound,
		},
		{
			name:           "無い確認",
			invitationID:   testutil.NewInvitationBuilder(t, db).Build(),
			confirmationID: model.EmailConfirmationID(uuid.New()),
			wantCode:       model.AppErrCodeResourceNotFound,
		},
	}
	for _, tt := range tests {
		invitationID := tt.invitationID
		confirmationID := tt.confirmationID
		if confirmationID == (model.EmailConfirmationID{}) {
			confirmationID, _ = confirmedEmail(t)
		}
		atname := tt.atname
		if atname == "" {
			atname = testutil.UniqueAtname()
		}

		_, err := uc.Execute(ctx, usecase.CreateAccountInput{
			InvitationID:        invitationID,
			EmailConfirmationID: confirmationID,
			Atname:              atname,
			Password:            accountPassword,
			Locale:              model.LocaleJa,
		})

		if tt.wantValidation {
			if model.AsValidationError(err) == nil {
				t.Errorf("%s: エラー = %v、ValidationErrorを期待", tt.name, err)
			}
		} else if ae := model.AsAppError(err); ae == nil || ae.Code != tt.wantCode {
			t.Errorf("%s: エラー = %v、コード %d のAppErrorを期待", tt.name, err, tt.wantCode)
		}
		if user, err := repository.NewUserRepository(db).FindByAtname(ctx, atname); err != nil || user != nil {
			t.Errorf("%s: ユーザー = (%+v, %v)、作成しないことを期待", tt.name, user, err)
		}
		if tt.wantConfirmation {
			if confirmation, err := repository.NewEmailConfirmationRepository(db).FindConfirmedByID(ctx, confirmationID); err != nil || confirmation == nil {
				t.Errorf("%s: 確認 = (%+v, %v)、残すことを期待", tt.name, confirmation, err)
			}
		}
	}
}

// TestCreateAccountUsecase_Execute_Concurrent は、同じ招待や同じアットネームで同時にアカウントを作っても、
// 人数の上限や一意性を超えて作れず、残りは招待が使えない・アットネームが使われているエラーになることを検証する。
func TestCreateAccountUsecase_Execute_Concurrent(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	adminInvitationID := testutil.NewInvitationBuilder(t, db).Build()
	const inviterRemaining = 2
	inviterInvitationID := inviterInvitationWithRemaining(t, inviterRemaining)
	sharedAtname := testutil.UniqueAtname()
	invitationUnusable := func(err error) bool {
		ae := model.AsAppError(err)
		return ae != nil && ae.Code == model.AppErrCodeForbidden
	}

	tests := []struct {
		name  string
		input func() usecase.CreateAccountInput
		// rejected は、作れなかった側のエラーが期待どおりかを返す。
		rejected    func(err error) bool
		wantCreated int
	}{
		{
			name: "同じ管理者の招待",
			input: func() usecase.CreateAccountInput {
				return usecase.CreateAccountInput{InvitationID: adminInvitationID, Atname: testutil.UniqueAtname()}
			},
			rejected:    invitationUnusable,
			wantCreated: 1,
		},
		{
			name: "人数の残りが2人の招待者の招待",
			input: func() usecase.CreateAccountInput {
				return usecase.CreateAccountInput{InvitationID: inviterInvitationID, Atname: testutil.UniqueAtname()}
			},
			rejected:    invitationUnusable,
			wantCreated: inviterRemaining,
		},
		{
			name: "同じアットネーム",
			input: func() usecase.CreateAccountInput {
				return usecase.CreateAccountInput{InvitationID: testutil.NewInvitationBuilder(t, db).Build(), Atname: sharedAtname}
			},
			rejected: func(err error) bool {
				ve := model.AsValidationError(err)
				return ve != nil && ve.HasFieldError("atname")
			},
			wantCreated: 1,
		},
	}
	for _, tt := range tests {
		const attempts = 4
		inputs := make([]usecase.CreateAccountInput, attempts)
		for i := range inputs {
			inputs[i] = tt.input()
			inputs[i].EmailConfirmationID, _ = confirmedEmail(t)
			inputs[i].Password = accountPassword
			inputs[i].Locale = model.LocaleJa
		}

		uc := newCreateAccountUsecase()
		ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
		errs := make([]error, attempts)
		var wg sync.WaitGroup
		for i := range inputs {
			wg.Go(func() {
				_, errs[i] = uc.Execute(ctx, inputs[i])
			})
		}
		wg.Wait()

		created := 0
		for _, err := range errs {
			switch {
			case err == nil:
				created++
			case !tt.rejected(err):
				t.Errorf("%s: 作れなかった側のエラー = %v", tt.name, err)
			}
		}
		if created != tt.wantCreated {
			t.Errorf("%s: 作れたアカウントの数 = %d、期待値 = %d", tt.name, created, tt.wantCreated)
		}
	}
}

// inviterInvitationWithRemaining は、招待できる人数の残りが remaining 人の招待者の、使える招待のIDを返す。
// 使用の記録の1件は取り消した招待に付け、招待者のすべての招待を合わせて数えることも確かめられるようにする。
func inviterInvitationWithRemaining(t *testing.T, remaining int) model.InvitationID {
	t.Helper()

	db := testutil.GetTestDB()
	inviterID := testutil.NewUserBuilder(t, db).Build()
	revokedID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(inviterID).WithRevokedAt(time.Now()).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, revokedID).Build()

	currentID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(inviterID).Build()
	for range model.InviterRedemptionLimit - remaining - 1 {
		testutil.NewInvitationRedemptionBuilder(t, db, currentID).Build()
	}

	return currentID
}
