package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestRunUser_Usage は、操作が無い / 未知 / ユーザーの指定が無いときに使い方を表示して失敗することを検証する。
func TestRunUser_Usage(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{}, {"delete", "alice"}, {"disable-two-factor"}, {"disable-two-factor", "alice", "extra"}} {
		var stdout, stderr bytes.Buffer

		if code := runUser(args, &stdout, &stderr); code != exitUsage {
			t.Errorf("runUser(%q)の終了コード = %d、期待値 = %d", args, code, exitUsage)
		}
		if !strings.Contains(stderr.String(), "使い方: cutre user <操作>") {
			t.Errorf("runUser(%q)の標準エラー出力 = %q、使い方を期待", args, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("runUser(%q)の標準出力 = %q、空を期待", args, stdout.String())
		}
	}
}

// TestRun_UserDisableTwoFactor は、アットネームとメールアドレスで指定したユーザーの二要素認証を無効にし、
// 有効にしていないユーザーではそのことを伝えて成功し、見つからないユーザーでは失敗することを検証する。
// t.Setenv を使うため t.Parallel() は呼ばない。
func TestRun_UserDisableTwoFactor(t *testing.T) {
	setCommandEnv(t)

	ctx := context.Background()
	db := testutil.GetTestDB()
	twoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	recoveryCodeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)

	atname := testutil.UniqueAtname()
	email := testutil.UniqueEmail("disable-two-factor")
	byAtname := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	byEmail := testutil.NewUserBuilder(t, db).WithEmail(email).Build()
	for _, userID := range []model.UserID{byAtname, byEmail} {
		testutil.NewUserTwoFactorAuthBuilder(t, db, userID).WithEnabledAt(time.Now()).Build()
		if err := recoveryCodeRepo.CreateAll(ctx, userID, []string{"digest-1"}); err != nil {
			t.Fatalf("リカバリーコードの作成のエラー = %v", err)
		}
	}

	for identifier, userID := range map[string]model.UserID{"@" + atname: byAtname, strings.ToUpper(email): byEmail} {
		var stdout bytes.Buffer
		if code := runUserDisableTwoFactor(identifier, &stdout); code != 0 {
			t.Fatalf("%s: 終了コード = %d、期待値 = 0", identifier, code)
		}
		if !strings.HasSuffix(stdout.String(), "の二要素認証を無効にしました\n") {
			t.Errorf("%s: 標準出力 = %q、無効にしたことを期待", identifier, stdout.String())
		}
		if setting, err := twoFactorAuthRepo.FindByUserID(ctx, userID); err != nil || setting != nil {
			t.Errorf("%s: 無効にした後の設定 = (%+v, %v)、(nil, nil) を期待", identifier, setting, err)
		}
		if count, err := recoveryCodeRepo.CountUnused(ctx, userID); err != nil || count != 0 {
			t.Errorf("%s: 無効にした後のリカバリーコード = (%d, %v)、(0, nil) を期待", identifier, count, err)
		}
	}

	var stdout bytes.Buffer
	if code := runUserDisableTwoFactor(atname, &stdout); code != 0 {
		t.Fatalf("有効にしていないユーザーの終了コード = %d、期待値 = 0", code)
	}
	if want := "@" + atname + " は二要素認証を有効にしていません\n"; stdout.String() != want {
		t.Errorf("有効にしていないユーザーの標準出力 = %q、期待値 = %q", stdout.String(), want)
	}

	stdout.Reset()
	if code := runUserDisableTwoFactor(testutil.UniqueAtname(), &stdout); code != 1 {
		t.Errorf("見つからないユーザーの終了コード = %d、期待値 = 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("見つからないユーザーの標準出力 = %q、空を期待", stdout.String())
	}
}
