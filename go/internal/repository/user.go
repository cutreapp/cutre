// Package repository はsqlcが生成したクエリの結果をドメインモデルに変換する。
// 1つのリポジトリが1つのモデルを担当し (model.User ↔ UserRepository)、
// データベースの詳細を上位の層から隠す。
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// UserRepository はusersを読み書きする。
type UserRepository struct {
	q *query.Queries
}

// NewUserRepository は UserRepository を生成する。
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい UserRepository を返す。
// レシーバ自身は変わらないため、UseCaseはトランザクションの境界だけを切り替えられる。
func (r *UserRepository) WithTx(tx *sql.Tx) *UserRepository {
	return &UserRepository{q: r.q.WithTx(tx)}
}

// FindByID は指定したIDのユーザーを返す。存在しない場合は (nil, nil) を返す。
// 未存在は正常なルックアップの結果であり、業務上の失敗として扱うかは呼び出し側が決める。
func (r *UserRepository) FindByID(ctx context.Context, id model.UserID) (*model.User, error) {
	row, err := r.q.GetUserByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return toUserModel(row), nil
}

// LockByID は指定したIDのユーザーの行をトランザクションが終わるまでロックして返す。存在しない場合は (nil, nil) を返す。
// 同じユーザーのパスワードリセットのトークンの置換や招待の作成を直列化し、ロックを取った時点の退会の有無と宛先で処理するために使う。
// 退会 (Withdraw) の更新と競合するため、退会の途中に届いた処理は退会のコミットを待ち、退会したユーザーとして (nil, nil) を受け取る。
func (r *UserRepository) LockByID(ctx context.Context, id model.UserID) (*model.User, error) {
	row, err := r.q.LockUserByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return toUserModel(row), nil
}

// FindByEmail は指定したメールアドレスのユーザーを返す。存在しない場合は (nil, nil) を返す。
// email列はcitextのため、照合は大文字小文字を無視する。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return toUserModel(row), nil
}

// FindByAtname は指定したアットネームのユーザーを返す。存在しない場合は (nil, nil) を返す。
// atname列はcitextのため、照合はUNIQUE制約と同じく大文字小文字を無視する。
func (r *UserRepository) FindByAtname(ctx context.Context, atname string) (*model.User, error) {
	row, err := r.q.GetUserByAtname(ctx, atname)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return toUserModel(row), nil
}

// CreateUserInput はユーザーの作成に必要な属性。
// idとタイムスタンプはデータベースが採番する。
type CreateUserInput struct {
	Email    string
	Atname   string
	Locale   model.Locale
	TimeZone string
}

// Create はユーザーを挿入し、データベースが採番したidとタイムスタンプを含めて返す。
//
// メールアドレスかアットネームが他のユーザーと重なったときは、ErrUserEmailTaken / ErrUserAtnameTaken を返す。
// 空きを確かめてから挿入するまでの間に、同じ値で先に登録されうるため。
func (r *UserRepository) Create(ctx context.Context, input CreateUserInput) (*model.User, error) {
	row, err := r.q.CreateUser(ctx, query.CreateUserParams{
		Email:    input.Email,
		Atname:   input.Atname,
		Locale:   string(input.Locale),
		TimeZone: input.TimeZone,
	})
	if err != nil {
		switch uniqueViolationConstraint(err) {
		case "users_email_key":
			return nil, ErrUserEmailTaken
		case "users_atname_key":
			return nil, ErrUserAtnameTaken
		}
		return nil, err
	}

	return toUserModel(row), nil
}

// Withdraw はユーザーを退会した状態にし、メールアドレスとアットネームを匿名の値 (anonymizedEmail・anonymizedAtname) に置き換える。
// ロケールとタイムゾーンも全員共通の値に置き換え、退会前の属性を残さない。
// 行は消さずに残し、招待の経路を辿れるようにする。既に退会していて更新しなかったときはfalseを返す。
func (r *UserRepository) Withdraw(ctx context.Context, id model.UserID, anonymizedEmail, anonymizedAtname string) (bool, error) {
	affected, err := r.q.WithdrawUser(ctx, query.WithdrawUserParams{
		ID:     uuid.UUID(id),
		Email:  anonymizedEmail,
		Atname: anonymizedAtname,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// toUserModel はusersの行を model.User に変換する。
// 生のuuidを型付きのIDに変換するのはこの境界だけで、上位の層はuuidを意識しない。
//
// リポジトリのメソッドではなく関数にしているのは、ユーザーを結合して引く他のリポジトリ
// (UserSessionRepository など) が、UserRepositoryを組み立てずに同じ変換を使えるようにするため。
func toUserModel(row query.User) *model.User {
	return &model.User{
		ID:        model.UserID(row.ID),
		Email:     row.Email,
		Atname:    row.Atname,
		Locale:    model.Locale(row.Locale),
		TimeZone:  row.TimeZone,
		DeletedAt: row.DeletedAt,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
