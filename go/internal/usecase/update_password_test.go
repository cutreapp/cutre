package usecase_test

import (
	"context"
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

const newPassword = "new-password1234"

// newUpdatePasswordUsecase はテスト用のデータベースに直接書き込む UpdatePasswordUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newUpdatePasswordUsecase() *usecase.UpdatePasswordUsecase {
	db := testutil.GetTestDB()

	return usecase.NewUpdatePasswordUsecase(
		db,
		repository.NewPasswordResetTokenRepository(db),
		validator.NewPasswordUpdateValidator(),
		repository.NewUserPasswordRepository(db),
		repository.NewUserSessionRepository(db),
	)
}

// userWithPassword はパスワードを持つユーザーを作り、そのIDを返す。
func userWithPassword(t *testing.T) model.UserID {
	t.Helper()

	db := testutil.GetTestDB()
	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).Build()

	return userID
}

// countRows はテーブルのうち、指定したユーザーの行の件数を返す。
func countRows(t *testing.T, table string, userID model.UserID) int {
	t.Helper()

	var count int
	// テーブル名はテストが渡す定数に限るため、クエリに埋め込んでよい。
	if err := testutil.GetTestDB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM "+table+" WHERE user_id = $1", uuid.UUID(userID),
	).Scan(&count); err != nil {
		t.Fatalf("%sの件数の取得のエラー = %v", table, err)
	}

	return count
}

// assertPassword はユーザーの保存したパスワードがpasswordと一致することを確かめる。
func assertPassword(t *testing.T, userID model.UserID, password string) {
	t.Helper()

	stored, err := repository.NewUserPasswordRepository(testutil.GetTestDB()).FindByUserID(context.Background(), userID)
	if err != nil || stored == nil {
		t.Fatalf("パスワードの取得 = (%v, %v)、パスワードを期待", stored, err)
	}
	if err := auth.CheckPassword(stored.PasswordDigest, password); err != nil {
		t.Errorf("保存したパスワードが %q と一致しない: %v", password, err)
	}
}

// TestUpdatePasswordUsecase_Execute は、パスワードを置き換え、トークンとそのユーザーのセッションをすべて消し、
// ほかのユーザーのセッションは残すことを検証する。
func TestUpdatePasswordUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	userID := userWithPassword(t)
	tokenID := testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(userID).Build()
	for range 2 {
		testutil.NewUserSessionBuilder(t, db).WithUserID(userID).Build()
	}
	otherID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserSessionBuilder(t, db).WithUserID(otherID).Build()

	err := newUpdatePasswordUsecase().Execute(context.Background(), usecase.UpdatePasswordInput{
		PasswordResetTokenID: tokenID,
		Password:             newPassword,
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	assertPassword(t, userID, newPassword)
	if got := countRows(t, "password_reset_tokens", userID); got != 0 {
		t.Errorf("トークンの件数 = %d、期待値 = 0", got)
	}
	if got := countRows(t, "user_sessions", userID); got != 0 {
		t.Errorf("セッションの件数 = %d、期待値 = 0", got)
	}
	if got := countRows(t, "user_sessions", otherID); got != 1 {
		t.Errorf("ほかのユーザーのセッションの件数 = %d、期待値 = 1", got)
	}
}

// TestUpdatePasswordUsecase_Execute_InvalidPassword は、パスワードの誤りではトークンもセッションも残し、
// 同じリンクで送り直せるようにすることを検証する。
func TestUpdatePasswordUsecase_Execute_InvalidPassword(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	userID := userWithPassword(t)
	tokenID := testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(userID).Build()
	testutil.NewUserSessionBuilder(t, db).WithUserID(userID).Build()

	err := newUpdatePasswordUsecase().Execute(i18n.SetLocale(context.Background(), i18n.LangJa), usecase.UpdatePasswordInput{
		PasswordResetTokenID: tokenID,
		Password:             "short",
	})

	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("password") {
		t.Fatalf("エラー = %v、passwordのフィールドエラーを期待", err)
	}
	assertPassword(t, userID, testutil.DefaultBuilderPassword)
	if got := countRows(t, "password_reset_tokens", userID); got != 1 {
		t.Errorf("トークンの件数 = %d、期待値 = 1", got)
	}
	if got := countRows(t, "user_sessions", userID); got != 1 {
		t.Errorf("セッションの件数 = %d、期待値 = 1", got)
	}
}

// TestUpdatePasswordUsecase_Execute_UnusableToken は、期限切れ・使用済みのトークンではパスワードを変えず、
// AppErrCodeResourceNotFound を返すことを検証する。
func TestUpdatePasswordUsecase_Execute_UnusableToken(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	expiredUserID := userWithPassword(t)
	expiredID := testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(expiredUserID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()

	usedUserID := userWithPassword(t)
	usedID := testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(usedUserID).Build()
	uc := newUpdatePasswordUsecase()
	if err := uc.Execute(context.Background(), usecase.UpdatePasswordInput{PasswordResetTokenID: usedID, Password: newPassword}); err != nil {
		t.Fatalf("1回目のExecute()のエラー = %v", err)
	}

	tests := []struct {
		name    string
		userID  model.UserID
		tokenID model.PasswordResetTokenID
		want    string
	}{
		{name: "期限切れのトークン", userID: expiredUserID, tokenID: expiredID, want: testutil.DefaultBuilderPassword},
		{name: "使用済みのトークン", userID: usedUserID, tokenID: usedID, want: newPassword},
	}
	for _, tt := range tests {
		err := uc.Execute(context.Background(), usecase.UpdatePasswordInput{PasswordResetTokenID: tt.tokenID, Password: "another-password"})

		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFoundを期待", tt.name, err)
		}
		assertPassword(t, tt.userID, tt.want)
	}
}
