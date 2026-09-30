package testutil

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// queryRower はビルダーが行を挿入する先。
// *sql.DB と *sql.Tx のどちらも満たすため、トランザクションで包むテストと、
// 自前でトランザクションを管理するテストの両方から同じビルダーを使える。
type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// UserBuilder はテスト用のusersの行を組み立てる。
// 既定値を持つため、テストは関心のあるフィールドだけを設定すればよい。
type UserBuilder struct {
	t         *testing.T
	db        queryRower
	email     string
	atname    string
	locale    model.Locale
	timeZone  string
	role      model.UserRole
	deletedAt *time.Time
}

// NewUserBuilder は UserBuilder を生成する。
// 既定のメールアドレスとアットネームは他と重複しない値のため、
// 複数のユーザーを作るテストが値を1つずつ決める必要はない。
func NewUserBuilder(t *testing.T, db queryRower) *UserBuilder {
	t.Helper()

	return &UserBuilder{
		t:        t,
		db:       db,
		email:    UniqueEmail("test"),
		atname:   UniqueAtname(),
		locale:   model.DefaultLocale,
		timeZone: "Asia/Tokyo",
		role:     model.UserRoleUser,
	}
}

// WithEmail はメールアドレスを設定する。
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.email = email
	return b
}

// WithAtname はアットネームを設定する。
func (b *UserBuilder) WithAtname(atname string) *UserBuilder {
	b.atname = atname
	return b
}

// WithLocale はロケールを設定する。
func (b *UserBuilder) WithLocale(locale model.Locale) *UserBuilder {
	b.locale = locale
	return b
}

// WithTimeZone はタイムゾーンを設定する。
func (b *UserBuilder) WithTimeZone(timeZone string) *UserBuilder {
	b.timeZone = timeZone
	return b
}

// WithRole は役割を設定する。未設定なら一般のユーザーを作る。
func (b *UserBuilder) WithRole(role model.UserRole) *UserBuilder {
	b.role = role
	return b
}

// WithDeletedAt は指定した時刻で退会したユーザーにする。
// 未設定なら在籍中のユーザー (deleted_atがNULL) を作る。
func (b *UserBuilder) WithDeletedAt(deletedAt time.Time) *UserBuilder {
	b.deletedAt = &deletedAt
	return b
}

// Build はユーザーを挿入し、データベースが採番したIDを返す。
// idとタイムスタンプはデータベースの既定値に任せる。
func (b *UserBuilder) Build() model.UserID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO users (email, atname, locale, time_zone, role, deleted_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		b.email, b.atname, string(b.locale), b.timeZone, string(b.role), b.deletedAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ユーザーの作成に失敗しました: %v", err)
	}

	return model.UserID(id)
}

// WithdrawUser は作ったユーザーを、退会と同じくアットネームとメールアドレスを匿名の値に置き換えて退会したことにする。
// 交換の相手が後から退会した状態を作るのに使う。
func WithdrawUser(t *testing.T, db queryRower, id model.UserID) {
	t.Helper()

	var withdrawnID uuid.UUID
	err := db.QueryRowContext(context.Background(),
		`UPDATE users SET deleted_at = NOW(), email = $2, atname = $3 WHERE id = $1 RETURNING id`,
		uuid.UUID(id), model.AnonymizedEmail(id), model.AnonymizedAtname(id),
	).Scan(&withdrawnID)
	if err != nil {
		t.Fatalf("テスト用ユーザーの退会に失敗しました: %v", err)
	}
}
