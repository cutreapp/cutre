// Package seed は開発環境へ、画面を確かめるためのアカウントを作る。
//
// 作るアカウントは名簿のファイル (go/seed-users.toml) で決める。
// 名簿は開発者が自分でメールを読めるアドレスを持つためバージョン管理に入れず、
// 形式は見本 (go/seed-users.example.toml) で示す。
package seed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"github.com/BurntSushi/toml"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// RosterPath は名簿のファイルの、go/ からの相対パス。
const RosterPath = "seed-users.toml"

// Roster は名簿のファイルの内容。
type Roster struct {
	// Password はすべてのアカウントで共通のパスワード。
	// シードは開発環境でしか動かないため、アカウントごとに分けても守れるものが無い。
	Password string       `toml:"password"`
	Users    []RosterUser `toml:"users"`
}

// RosterUser は名簿が挙げるアカウント1件。
type RosterUser struct {
	Atname string `toml:"atname"`
	Email  string `toml:"email"`
	// Locale は省くと日本語になる。
	Locale string `toml:"locale"`
}

// LoadRoster は名簿のファイルを読む。
//
// 知らないキーはエラーにする。綴りを誤った設定が黙って無視されると、
// 意図と違うアカウントができたことに気付きにくいため。
func LoadRoster(path string) (*Roster, error) {
	var roster Roster
	meta, err := toml.DecodeFile(path, &roster)
	if err != nil {
		return nil, fmt.Errorf("名簿 %s を読めません: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("名簿 %s に知らないキーがあります: %v", path, undecoded)
	}

	if err := auth.ValidatePasswordStrength(roster.Password); err != nil {
		return nil, fmt.Errorf("名簿 %s の password がパスワードのポリシーを満たしません: %w", path, err)
	}
	for i, user := range roster.Users {
		if user.Atname == "" || user.Email == "" {
			return nil, fmt.Errorf("名簿 %s の %d件目のアカウントに atname と email がありません", path, i+1)
		}
		if user.Locale != "" {
			if _, ok := model.ParseLocale(user.Locale); !ok {
				return nil, fmt.Errorf("名簿 %s の %d件目のアカウントの locale が読めません: %q", path, i+1, user.Locale)
			}
		}
	}

	return &roster, nil
}

// CreateUsers は名簿のアカウントのうち、まだ無いものを作る。作ったものと飛ばしたものを out に書く。
//
// 同じメールアドレスのアカウントが既にあれば作らない。何度実行しても同じ状態に落ち着き、
// 名簿にアカウントを書き足してから再実行すれば、足した分だけが作られる。
// 1件ずつトランザクションで包み、ユーザーだけがあってパスワードの無いアカウントを残さない。
func CreateUsers(ctx context.Context, db *sql.DB, roster *Roster, out io.Writer) error {
	digest, err := auth.HashPassword(roster.Password)
	if err != nil {
		return fmt.Errorf("パスワードのハッシュ化に失敗: %w", err)
	}

	userRepo := repository.NewUserRepository(db)
	for _, user := range roster.Users {
		existing, err := userRepo.FindByEmail(ctx, user.Email)
		if err != nil {
			return fmt.Errorf("%s の確認に失敗: %w", user.Email, err)
		}
		if existing != nil {
			_, _ = fmt.Fprintf(out, "既にあります: @%s (%s)\n", existing.Atname, existing.Email)
			continue
		}

		if err := createUser(ctx, db, user, digest); err != nil {
			return fmt.Errorf("@%s の作成に失敗: %w", user.Atname, err)
		}
		_, _ = fmt.Fprintf(out, "作成しました: @%s (%s)\n", user.Atname, user.Email)
	}

	return nil
}

// createUser はアカウント1件を、ユーザーとパスワードの組で作る。
func createUser(ctx context.Context, db *sql.DB, user RosterUser, passwordDigest string) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	locale := model.LocaleJa
	if parsed, ok := model.ParseLocale(user.Locale); ok {
		locale = parsed
	}

	created, err := repository.NewUserRepository(db).WithTx(tx).Create(ctx, repository.CreateUserInput{
		Email:    user.Email,
		Atname:   user.Atname,
		Locale:   locale,
		TimeZone: model.DefaultTimeZone,
	})
	if err != nil {
		return err
	}

	if _, err := repository.NewUserPasswordRepository(db).WithTx(tx).Create(ctx, repository.CreateUserPasswordInput{
		UserID:         created.ID,
		PasswordDigest: passwordDigest,
	}); err != nil {
		return err
	}

	return tx.Commit()
}
