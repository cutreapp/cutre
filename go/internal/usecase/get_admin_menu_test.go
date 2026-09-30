package usecase_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newUserWithRole は role の役割のユーザーを作り、UseCaseに渡すモデルとして読み戻す。
func newUserWithRole(t *testing.T, tx *sql.Tx, role model.UserRole) *model.User {
	t.Helper()

	id := testutil.NewUserBuilder(t, tx).WithRole(role).Build()
	user, err := repository.NewUserRepository(testutil.GetTestDB()).WithTx(tx).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// newCommittedUserWithRole は newUserWithRole と同じく role の役割のユーザーを作るが、テストのトランザクションを使わずにコミットする。
// 自分でトランザクションを開くUseCaseのテストで使う。
func newCommittedUserWithRole(t *testing.T, role model.UserRole) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	id := testutil.NewUserBuilder(t, db).WithRole(role).Build()
	user, err := repository.NewUserRepository(db).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// assertAppErrorCode は err が code の *model.AppError であることを検証する。
func assertAppErrorCode(t *testing.T, err error, code model.AppErrorCode) {
	t.Helper()

	if ae := model.AsAppError(err); ae == nil || ae.Code != code {
		t.Errorf("エラー = %v、AppErrorCode %d を期待", err, code)
	}
}

// jaContext は翻訳を日本語で引くcontextを返す。バリデーションのメッセージを確かめるテストが使う。
func jaContext() context.Context {
	return i18n.SetLocale(context.Background(), i18n.LangJa)
}

// TestGetAdminMenuUsecase_Execute は、管理画面の入口を編集者と管理者にだけ開かせ、
// 一般のユーザーとログインしていないときは AppErrCodeForbidden を返すことを検証する。
func TestGetAdminMenuUsecase_Execute(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	uc := usecase.NewGetAdminMenuUsecase()

	for _, role := range []model.UserRole{model.UserRoleEditor, model.UserRoleAdmin} {
		if err := uc.Execute(t.Context(), usecase.GetAdminMenuInput{User: newUserWithRole(t, tx, role)}); err != nil {
			t.Errorf("%s: Execute()のエラー = %v、nilを期待", role, err)
		}
	}
	assertAppErrorCode(t, uc.Execute(t.Context(), usecase.GetAdminMenuInput{User: newUserWithRole(t, tx, model.UserRoleUser)}), model.AppErrCodeForbidden)
	assertAppErrorCode(t, uc.Execute(t.Context(), usecase.GetAdminMenuInput{User: nil}), model.AppErrCodeForbidden)
}
