package usecase_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newDeleteAccountUsecase はテスト用のデータベースに直接書き込む DeleteAccountUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newDeleteAccountUsecase() *usecase.DeleteAccountUsecase {
	db := testutil.GetTestDB()
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	return usecase.NewDeleteAccountUsecase(
		db,
		validator.NewWithdrawalDeleteValidator(userPasswordRepo, repository.NewTradeRepository(db)),
		repository.NewUserRepository(db),
		userPasswordRepo,
		repository.NewUserSessionRepository(db),
		repository.NewUserTwoFactorAuthRepository(db),
		repository.NewUserTwoFactorRecoveryCodeRepository(db),
		repository.NewPasswordResetTokenRepository(db),
		repository.NewEmailConfirmationRepository(db),
		repository.NewInvitationRepository(db),
		repository.NewItemRepository(db),
		repository.NewUserStationRepository(db),
	)
}

// countQuery は件数を数えるクエリを実行して、その件数を返す。
func countQuery(t *testing.T, sqlText string, arg any) int {
	t.Helper()

	var count int
	if err := testutil.GetTestDB().QueryRowContext(context.Background(), sqlText, arg).Scan(&count); err != nil {
		t.Fatalf("行の数の取得のエラー (%s) = %v", sqlText, err)
	}

	return count
}

// TestDeleteAccountUsecase_Execute は、退会でusersの行を匿名にして残し、認証の情報とメールアドレスへの確認を消し、
// 取り消していない招待を取り消すこと、招待の経路 (招待の行と使用の記録) は残すことを検証する。
func TestDeleteAccountUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	db := testutil.GetTestDB()

	// 退会するユーザーは、招待で登録し、自分も招待を持つ。
	adminInvitationID := testutil.NewInvitationBuilder(t, db).Build()
	user := twoFactorUser(t)
	testutil.NewInvitationRedemptionBuilder(t, db, adminInvitationID).WithUserID(user.ID).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(user.ID).Build()
	testutil.NewUserSessionBuilder(t, db).WithUserID(user.ID).Build()
	testutil.NewUserSessionBuilder(t, db).WithUserID(user.ID).Build()
	enableTwoFactorAuth(t, newTestTwoFactorKey(t), user.ID)
	if err := repository.NewUserTwoFactorRecoveryCodeRepository(db).CreateAll(ctx, user.ID, []string{"digest-1", "digest-2"}); err != nil {
		t.Fatalf("リカバリーコードの作成のエラー = %v", err)
	}
	testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(user.ID).Build()
	testutil.NewEmailConfirmationBuilder(t, db).WithEmail(user.Email).Build()
	unrevokedID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, unrevokedID).Build()

	// ほかのユーザーの情報は変わらない。
	other := twoFactorUser(t)
	testutil.NewUserPasswordBuilder(t, db).WithUserID(other.ID).Build()
	testutil.NewUserSessionBuilder(t, db).WithUserID(other.ID).Build()
	otherInvitationID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(other.ID).Build()

	err := newDeleteAccountUsecase().Execute(ctx, usecase.DeleteAccountInput{User: user, CurrentPassword: testutil.DefaultBuilderPassword, Confirmed: true})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	id := uuid.UUID(user.ID)
	var email, atname string
	var deletedAt *time.Time
	if err := db.QueryRowContext(ctx, "SELECT email, atname, deleted_at FROM users WHERE id = $1", id).Scan(&email, &atname, &deletedAt); err != nil {
		t.Fatalf("退会したユーザーの行の取得のエラー = %v", err)
	}
	if email != model.AnonymizedEmail(user.ID) || atname != model.AnonymizedAtname(user.ID) || deletedAt == nil {
		t.Errorf("退会したユーザーの行 = (%q, %q, %v)、匿名の値と退会した時刻を期待", email, atname, deletedAt)
	}

	for _, table := range []string{"user_passwords", "user_sessions", "user_two_factor_auths", "user_two_factor_recovery_codes", "password_reset_tokens"} {
		if got := countRows(t, table, user.ID); got != 0 {
			t.Errorf("%sの行の数 = %d、期待値 = 0", table, got)
		}
	}
	if got := countQuery(t, "SELECT COUNT(*) FROM invitations WHERE inviter_user_id = $1 AND revoked_at IS NULL", id); got != 0 {
		t.Errorf("取り消していない招待の数 = %d、期待値 = 0", got)
	}
	if got := countQuery(t, "SELECT COUNT(*) FROM email_confirmations WHERE email = $1", user.Email); got != 0 {
		t.Errorf("メールアドレスへの確認の行の数 = %d、期待値 = 0", got)
	}
	if got := countQuery(t, "SELECT COUNT(*) FROM invitations WHERE inviter_user_id = $1", id); got != 1 {
		t.Errorf("招待の行の数 = %d、取り消した招待が残ることを期待", got)
	}
	if got := countQuery(t, "SELECT COUNT(*) FROM invitation_redemptions WHERE invitation_id = $1", uuid.UUID(unrevokedID)); got != 1 {
		t.Errorf("退会したユーザーの招待で登録した記録の数 = %d、期待値 = 1", got)
	}
	if got := countQuery(t, "SELECT COUNT(*) FROM invitation_redemptions WHERE user_id = $1", id); got != 1 {
		t.Errorf("退会したユーザー自身が招待で登録した記録の数 = %d、期待値 = 1", got)
	}

	if got := countRows(t, "user_passwords", other.ID) + countRows(t, "user_sessions", other.ID); got != 2 {
		t.Errorf("ほかのユーザーのパスワードとセッションの行の数 = %d、期待値 = 2", got)
	}
	if invitation, err := repository.NewInvitationRepository(db).FindByID(ctx, otherInvitationID); err != nil || invitation == nil || invitation.RevokedAt != nil {
		t.Errorf("ほかのユーザーの招待 = (%+v, %v)、取り消していないままを期待", invitation, err)
	}
}

// TestDeleteAccountUsecase_Execute_Invalid は、パスワードの誤りや未チェックでは *model.ValidationError を返し、退会させないことを検証する。
func TestDeleteAccountUsecase_Execute_Invalid(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	user := twoFactorUser(t)
	testutil.NewUserPasswordBuilder(t, testutil.GetTestDB()).WithUserID(user.ID).Build()

	for name, input := range map[string]usecase.DeleteAccountInput{
		"パスワードが違う": {User: user, CurrentPassword: "wrong-password", Confirmed: true},
		"チェックが無い":  {User: user, CurrentPassword: testutil.DefaultBuilderPassword},
	} {
		err := newDeleteAccountUsecase().Execute(ctx, input)
		if model.AsValidationError(err) == nil {
			t.Errorf("%s: Execute()のエラー = %v、ValidationErrorを期待", name, err)
		}
	}

	if found, err := repository.NewUserRepository(testutil.GetTestDB()).FindByID(ctx, user.ID); err != nil || found == nil {
		t.Errorf("ユーザー = (%+v, %v)、退会していないことを期待", found, err)
	}
}

// TestDeleteAccountUsecase_Execute_AlreadyWithdrawn は、パスワードを確かめた後に先の送信で退会していたとき、
// AppErrCodeConflict の *model.AppError を返すことを検証する。
func TestDeleteAccountUsecase_Execute_AlreadyWithdrawn(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	user := twoFactorUser(t)
	testutil.NewUserPasswordBuilder(t, testutil.GetTestDB()).WithUserID(user.ID).Build()
	withdrawn, err := repository.NewUserRepository(testutil.GetTestDB()).Withdraw(ctx, user.ID, model.AnonymizedEmail(user.ID), model.AnonymizedAtname(user.ID))
	if err != nil || !withdrawn {
		t.Fatalf("Withdraw() = (%t, %v)、(true, nil) を期待", withdrawn, err)
	}

	err = newDeleteAccountUsecase().Execute(ctx, usecase.DeleteAccountInput{User: user, CurrentPassword: testutil.DefaultBuilderPassword, Confirmed: true})
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
		t.Errorf("Execute()のエラー = %v、AppErrCodeConflict を期待", err)
	}
}

// TestDeleteAccountUsecase_Execute_AlreadyWithdrawnWithoutPassword は、先の退会でパスワードを削除した後も、
// 古いユーザーを持つ二重送信を入力エラーではなく退会済みとして扱うことを検証する。
func TestDeleteAccountUsecase_Execute_AlreadyWithdrawnWithoutPassword(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	user := twoFactorUser(t)
	testutil.NewUserPasswordBuilder(t, testutil.GetTestDB()).WithUserID(user.ID).Build()
	uc := newDeleteAccountUsecase()
	input := usecase.DeleteAccountInput{User: user, CurrentPassword: testutil.DefaultBuilderPassword, Confirmed: true}
	if err := uc.Execute(ctx, input); err != nil {
		t.Fatalf("先のExecute()のエラー = %v", err)
	}
	if got := countRows(t, "user_passwords", user.ID); got != 0 {
		t.Fatalf("退会後のパスワードの行の数 = %d、期待値 = 0", got)
	}

	err := uc.Execute(ctx, input)
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
		t.Errorf("後のExecute()のエラー = %v、AppErrCodeConflict を期待", err)
	}
}

// TestDeleteAccountUsecase_Execute_ListAndPlaces は、退会でリストのアイテムを行を残して外し、交換場所を消すこと、
// 終わった交換は退会を止めず、交換の記録とメッセージは残ることを検証する。
func TestDeleteAccountUsecase_Execute_ListAndPlaces(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	db := testutil.GetTestDB()
	user := twoFactorUser(t)
	testutil.NewUserPasswordBuilder(t, db).WithUserID(user.ID).Build()
	goodsID := testutil.NewGoodsBuilder(t, db, testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()).Build()
	giveID := testutil.NewItemBuilder(t, db, user.ID, goodsID).Build()
	testutil.NewItemBuilder(t, db, user.ID, goodsID).WithKind(model.ItemKindWant).Build()
	testutil.NewUserStationBuilder(t, db, user.ID, testutil.NewStationBuilder(t, db).Build()).Build()
	partnerID := testutil.NewUserBuilder(t, db).Build()
	tradeID := testutil.NewTradeBuilder(t, db, user.ID, partnerID).WithStatus(model.TradeStatusCompleted).WithEndedAt(time.Now()).Build()
	if err := repository.NewTradeItemRepository(db).CreateMany(ctx, tradeID, []model.ItemID{giveID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	testutil.NewTradeMessageBuilder(t, db, tradeID, user.ID, "よろしくお願いします").Build()

	err := newDeleteAccountUsecase().Execute(ctx, usecase.DeleteAccountInput{User: user, CurrentPassword: testutil.DefaultBuilderPassword, Confirmed: true})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	id := uuid.UUID(user.ID)
	if got := countQuery(t, "SELECT COUNT(*) FROM items WHERE user_id = $1 AND status = 'listed'", id); got != 0 {
		t.Errorf("リストにあるアイテムの数 = %d、期待値 = 0", got)
	}
	if got := countQuery(t, "SELECT COUNT(*) FROM items WHERE user_id = $1", id); got != 2 {
		t.Errorf("アイテムの行の数 = %d、外した行が残ることを期待", got)
	}
	if got := countRows(t, "user_stations", user.ID); got != 0 {
		t.Errorf("交換場所の行の数 = %d、期待値 = 0", got)
	}
	if got := countQuery(t, "SELECT COUNT(*) FROM trade_items WHERE trade_id = $1", uuid.UUID(tradeID)); got != 1 {
		t.Errorf("交換の品の行の数 = %d、期待値 = 1", got)
	}
	if got := countQuery(t, "SELECT COUNT(*) FROM trade_messages WHERE sender_user_id = $1", id); got != 1 {
		t.Errorf("退会したユーザーのメッセージの数 = %d、期待値 = 1", got)
	}
}

// TestDeleteAccountUsecase_Execute_TradeInProgress は、返事待ちかマッチ成立の交換があるとき、パスワードが合っていても
// フォーム全体の *model.ValidationError を返し、退会させず、リストと交換場所にも触れないことを検証する。
func TestDeleteAccountUsecase_Execute_TradeInProgress(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	db := testutil.GetTestDB()
	for _, status := range []model.TradeStatus{model.TradeStatusPending, model.TradeStatusMatched} {
		user := twoFactorUser(t)
		testutil.NewUserPasswordBuilder(t, db).WithUserID(user.ID).Build()
		goodsID := testutil.NewGoodsBuilder(t, db, testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()).Build()
		testutil.NewItemBuilder(t, db, user.ID, goodsID).Build()
		testutil.NewUserStationBuilder(t, db, user.ID, testutil.NewStationBuilder(t, db).Build()).Build()
		testutil.NewTradeBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), user.ID).WithStatus(status).Build()

		err := newDeleteAccountUsecase().Execute(ctx, usecase.DeleteAccountInput{User: user, CurrentPassword: testutil.DefaultBuilderPassword, Confirmed: true})
		ve := model.AsValidationError(err)
		if ve == nil || !slices.Equal(ve.Global, []string{"進行中の交換があるため、退会できません。交換がすべて終わってから、もう一度お試しください"}) {
			t.Errorf("%s: Execute()のエラー = %v、進行中の交換のValidationErrorを期待", status, err)
		}
		if found, err := repository.NewUserRepository(db).FindByID(ctx, user.ID); err != nil || found == nil {
			t.Errorf("%s: ユーザー = (%+v, %v)、退会していないことを期待", status, found, err)
		}
		id := uuid.UUID(user.ID)
		if got := countQuery(t, "SELECT COUNT(*) FROM items WHERE user_id = $1 AND status = 'listed'", id); got != 1 {
			t.Errorf("%s: リストにあるアイテムの数 = %d、期待値 = 1", status, got)
		}
		if got := countRows(t, "user_stations", user.ID); got != 1 {
			t.Errorf("%s: 交換場所の行の数 = %d、期待値 = 1", status, got)
		}
	}
}
