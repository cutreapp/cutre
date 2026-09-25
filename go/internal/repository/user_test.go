package repository_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestUserRepository_Create は、挿入したユーザーがデータベースの採番した値とともに
// モデルへ変換されて返ることを検証する。
func TestUserRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserRepository(db).WithTx(tx)

	email := testutil.UniqueEmail("create")
	atname := testutil.UniqueAtname()

	user, err := repo.Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   atname,
		Locale:   model.LocaleJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if user.ID == (model.UserID{}) {
		t.Error("ID = ゼロ値、データベースが採番した値を期待")
	}

	if user.Email != email {
		t.Errorf("Email = %q、期待値 = %q", user.Email, email)
	}

	if user.Atname != atname {
		t.Errorf("Atname = %q、期待値 = %q", user.Atname, atname)
	}

	if user.Locale != model.LocaleJa {
		t.Errorf("Locale = %q、期待値 = %q", user.Locale, model.LocaleJa)
	}

	if user.DeletedAt != nil {
		t.Errorf("DeletedAt = %v、期待値 = nil", user.DeletedAt)
	}

	if user.CreatedAt.IsZero() {
		t.Error("CreatedAt = ゼロ値、データベースが設定した時刻を期待")
	}
}

// TestUserRepository_FindByID は、IDでの取得が在籍中のユーザーだけを返すことを検証する。
func TestUserRepository_FindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserRepository(db).WithTx(tx)

	userID := testutil.NewUserBuilder(t, tx).Build()
	deletedUserID := testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()

	t.Run("在籍中のユーザーを取得できる", func(t *testing.T) {
		user, err := repo.FindByID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}

		if user == nil {
			t.Fatal("ユーザー = nil、非nilを期待")
		}

		if user.ID != userID {
			t.Errorf("ID = %s、期待値 = %s", user.ID, userID)
		}
	})

	t.Run("退会したユーザーは取得できない", func(t *testing.T) {
		user, err := repo.FindByID(ctx, deletedUserID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}

		if user != nil {
			t.Errorf("ユーザー = %v、期待値 = nil", user)
		}
	})

	t.Run("存在しないIDでは (nil, nil) を返す", func(t *testing.T) {
		user, err := repo.FindByID(ctx, model.UserID(uuid.New()))
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}

		if user != nil {
			t.Errorf("ユーザー = %v、期待値 = nil", user)
		}
	})
}

// TestUserRepository_LockByID は、在籍中のユーザーをロックして返し、退会したユーザーと存在しないIDでは (nil, nil) を返すことを検証する。
func TestUserRepository_LockByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserRepository(db).WithTx(tx)

	email := testutil.UniqueEmail("lock-by-id")
	userID := testutil.NewUserBuilder(t, tx).WithEmail(email).Build()
	deletedUserID := testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()

	user, err := repo.LockByID(ctx, userID)
	if err != nil {
		t.Fatalf("在籍中のユーザー: LockByID()のエラー = %v", err)
	}
	if user == nil || user.ID != userID || user.Email != email {
		t.Errorf("在籍中のユーザー: ユーザー = %v、ID %s・メールアドレス %q を期待", user, userID, email)
	}

	for name, id := range map[string]model.UserID{"退会したユーザー": deletedUserID, "存在しないID": model.UserID(uuid.New())} {
		user, err := repo.LockByID(ctx, id)
		if err != nil {
			t.Fatalf("%s: LockByID()のエラー = %v", name, err)
		}
		if user != nil {
			t.Errorf("%s: ユーザー = %v、期待値 = nil", name, user)
		}
	}
}

// TestUserRepository_FindByEmail は、メールアドレスでの取得が大文字小文字を無視し、
// 退会したユーザーを除外することを検証する。
func TestUserRepository_FindByEmail(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserRepository(db).WithTx(tx)

	email := testutil.UniqueEmail("find")
	userID := testutil.NewUserBuilder(t, tx).WithEmail(email).Build()

	deletedEmail := testutil.UniqueEmail("deleted")
	testutil.NewUserBuilder(t, tx).WithEmail(deletedEmail).WithDeletedAt(time.Now()).Build()

	t.Run("大文字小文字が違っても同じユーザーを取得できる", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, strings.ToUpper(email))
		if err != nil {
			t.Fatalf("FindByEmail()のエラー = %v", err)
		}

		if user == nil {
			t.Fatal("ユーザー = nil、非nilを期待")
		}

		if user.ID != userID {
			t.Errorf("ID = %s、期待値 = %s", user.ID, userID)
		}
	})

	t.Run("退会したユーザーは取得できない", func(t *testing.T) {
		user, err := repo.FindByEmail(ctx, deletedEmail)
		if err != nil {
			t.Fatalf("FindByEmail()のエラー = %v", err)
		}

		if user != nil {
			t.Errorf("ユーザー = %v、期待値 = nil", user)
		}
	})
}

// TestUserRepository_FindByAtname は、アットネームでの取得が大文字小文字を無視し、
// 退会したユーザーを除外することを検証する。
func TestUserRepository_FindByAtname(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserRepository(db).WithTx(tx)

	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, tx).WithAtname(atname).Build()

	deletedAtname := testutil.UniqueAtname()
	testutil.NewUserBuilder(t, tx).WithAtname(deletedAtname).WithDeletedAt(time.Now()).Build()

	t.Run("大文字小文字が違っても同じユーザーを取得できる", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, strings.ToUpper(atname))
		if err != nil {
			t.Fatalf("FindByAtname()のエラー = %v", err)
		}

		if user == nil {
			t.Fatal("ユーザー = nil、非nilを期待")
		}

		if user.ID != userID {
			t.Errorf("ID = %s、期待値 = %s", user.ID, userID)
		}
	})

	t.Run("退会したユーザーは取得できない", func(t *testing.T) {
		user, err := repo.FindByAtname(ctx, deletedAtname)
		if err != nil {
			t.Fatalf("FindByAtname()のエラー = %v", err)
		}

		if user != nil {
			t.Errorf("ユーザー = %v、期待値 = nil", user)
		}
	})
}

// TestUserRepository_Create_Taken は、メールアドレスかアットネームが他のユーザーと重なると、
// どちらが重なったかを表すエラーを返すことを検証する。大文字小文字だけが違う値も重なりとして扱う。
func TestUserRepository_Create_Taken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sameKey string
		wantErr error
	}{
		{name: "メールアドレス", sameKey: "email", wantErr: repository.ErrUserEmailTaken},
		{name: "アットネーム", sameKey: "atname", wantErr: repository.ErrUserAtnameTaken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 一意制約の違反はトランザクションを中断させるため、ケースごとにトランザクションを分ける。
			db, tx := testutil.SetupTx(t)
			repo := repository.NewUserRepository(db).WithTx(tx)
			email := testutil.UniqueEmail("taken")
			atname := testutil.UniqueAtname()
			testutil.NewUserBuilder(t, tx).WithEmail(email).WithAtname(atname).Build()

			input := repository.CreateUserInput{
				Email:    testutil.UniqueEmail("taken-other"),
				Atname:   testutil.UniqueAtname(),
				Locale:   model.LocaleJa,
				TimeZone: model.DefaultTimeZone,
			}
			if tt.sameKey == "email" {
				input.Email = strings.ToUpper(email)
			} else {
				input.Atname = strings.ToUpper(atname)
			}

			if _, err := repo.Create(context.Background(), input); !errors.Is(err, tt.wantErr) {
				t.Errorf("Create()のエラー = %v、期待値 = %v", err, tt.wantErr)
			}
		})
	}
}

// TestUserRepository_Withdraw は、退会した時刻を入れて個人の属性を共通の値に置き換え、
// 元の値を空けること、退会済みのユーザーは更新しないことを検証する。
func TestUserRepository_Withdraw(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserRepository(db).WithTx(tx)

	email := testutil.UniqueEmail("withdraw")
	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, tx).WithEmail(email).WithAtname(atname).WithLocale(model.LocaleEn).WithTimeZone("America/New_York").Build()
	anonymizedEmail := model.AnonymizedEmail(userID)
	anonymizedAtname := model.AnonymizedAtname(userID)

	withdrawn, err := repo.Withdraw(ctx, userID, anonymizedEmail, anonymizedAtname)
	if err != nil || !withdrawn {
		t.Fatalf("Withdraw() = (%t, %v)、(true, nil) を期待", withdrawn, err)
	}

	var gotEmail, gotAtname, gotLocale, gotTimeZone string
	var deletedAt *time.Time
	if err := tx.QueryRowContext(ctx, "SELECT email, atname, locale, time_zone, deleted_at FROM users WHERE id = $1", uuid.UUID(userID)).Scan(&gotEmail, &gotAtname, &gotLocale, &gotTimeZone, &deletedAt); err != nil {
		t.Fatalf("退会したユーザーの行の取得のエラー = %v", err)
	}
	if gotEmail != anonymizedEmail || gotAtname != anonymizedAtname || deletedAt == nil {
		t.Errorf("退会したユーザーの行 = (%q, %q, %v)、(%q, %q, 退会した時刻) を期待", gotEmail, gotAtname, deletedAt, anonymizedEmail, anonymizedAtname)
	}
	if gotLocale != string(model.LocaleJa) || gotTimeZone != "Etc/UTC" {
		t.Errorf("退会したユーザーのロケールとタイムゾーン = (%q, %q)、(ja, Etc/UTC) を期待", gotLocale, gotTimeZone)
	}

	// 空いたメールアドレスとアットネームで、別のユーザーが登録できる。
	if _, err := repo.Create(ctx, repository.CreateUserInput{Email: email, Atname: atname, Locale: model.LocaleJa, TimeZone: model.DefaultTimeZone}); err != nil {
		t.Errorf("元のメールアドレスとアットネームでのCreate()のエラー = %v", err)
	}

	withdrawn, err = repo.Withdraw(ctx, userID, anonymizedEmail, anonymizedAtname)
	if err != nil || withdrawn {
		t.Errorf("退会済みのユーザーのWithdraw() = (%t, %v)、(false, nil) を期待", withdrawn, err)
	}
}
