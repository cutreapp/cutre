package seed_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/seed"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// writeRoster はcontentを名簿のファイルとして一時ディレクトリへ書き、そのパスを返す。
func writeRoster(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "seed-users.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("名簿の書き込みのエラー = %v", err)
	}
	return path
}

// TestLoadRoster_Example は、コミットしている見本がそのまま名簿として読めることを検証する。
// 見本を複製して使う手順が、書き換える前の時点で壊れていないようにする。
func TestLoadRoster_Example(t *testing.T) {
	t.Parallel()

	roster, err := seed.LoadRoster("../../seed-users.example.toml")
	if err != nil {
		t.Fatalf("LoadRoster()のエラー = %v", err)
	}
	if len(roster.Users) == 0 {
		t.Error("アカウントが0件、1件以上を期待")
	}
}

// TestLoadRoster_Invalid は、意図と違うアカウントを作りうる名簿を読み込みの時点で拒むことを検証する。
func TestLoadRoster_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "知らないキー",
			content: "password = \"password\"\n[[users]]\natname = \"a\"\nemail = \"a@example.com\"\nrole = \"x\"\n",
			wantErr: "知らないキー",
		},
		{
			name:    "パスワードが短い",
			content: "password = \"short\"\n",
			wantErr: "ポリシー",
		},
		{
			name:    "メールアドレスが無い",
			content: "password = \"password\"\n[[users]]\natname = \"a\"\n",
			wantErr: "atname と email",
		},
		{
			name:    "読めないロケール",
			content: "password = \"password\"\n[[users]]\natname = \"a\"\nemail = \"a@example.com\"\nlocale = \"fr\"\n",
			wantErr: "locale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := seed.LoadRoster(writeRoster(t, tt.content))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("エラー = %v、%qを含むことを期待", err, tt.wantErr)
			}
		})
	}
}

// TestCreateUsers は、名簿のアカウントがそのパスワードでログインできる形で作られ、
// 再実行しても重複して作られないことを検証する。
//
// CreateUsers は自前でトランザクションを開くため、共有の接続で実行して行をコミットする。
// 値は実行ごとに一意にし、他のテストと衝突しないようにする。
func TestCreateUsers(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	ctx := context.Background()

	roster := &seed.Roster{
		Password: "password123",
		Users: []seed.RosterUser{
			{Atname: testutil.UniqueAtname(), Email: testutil.UniqueEmail("seed")},
			{Atname: testutil.UniqueAtname(), Email: testutil.UniqueEmail("seed"), Locale: "en"},
		},
	}

	var out strings.Builder
	if err := seed.CreateUsers(ctx, db, roster, &out); err != nil {
		t.Fatalf("CreateUsers()のエラー = %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	for i, want := range []model.Locale{model.LocaleJa, model.LocaleEn} {
		user, err := userRepo.FindByEmail(ctx, roster.Users[i].Email)
		if err != nil || user == nil {
			t.Fatalf("FindByEmail() = (%v, %v)、ユーザーを期待", user, err)
		}
		if user.Locale != want {
			t.Errorf("Locale = %q、期待値 = %q", user.Locale, want)
		}
		password, err := userPasswordRepo.FindByUserID(ctx, user.ID)
		if err != nil || password == nil {
			t.Fatalf("FindByUserID() = (%v, %v)、パスワードを期待", password, err)
		}
		if err := auth.CheckPassword(password.PasswordDigest, roster.Password); err != nil {
			t.Errorf("名簿のパスワードと照合できない: %v", err)
		}
	}

	out.Reset()
	if err := seed.CreateUsers(ctx, db, roster, &out); err != nil {
		t.Fatalf("2回目のCreateUsers()のエラー = %v", err)
	}
	if got := strings.Count(out.String(), "既にあります"); got != len(roster.Users) {
		t.Errorf("飛ばしたアカウント = %d件、期待値 = %d件\n出力: %s", got, len(roster.Users), out.String())
	}
}
