package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// UserRepository.Create が、一意であるべき値が他のユーザーと重なったときに返すエラー。
// 呼び出し側がドライバのエラーを調べずに、どの値が重なったかを見分けられるようにする。
var (
	ErrUserEmailTaken  = errors.New("メールアドレスは既に使われています")
	ErrUserAtnameTaken = errors.New("アットネームは既に使われています")
)

// uniqueViolationSQLState は、一意制約に違反したときにPostgreSQLが返すSQLSTATE。
const uniqueViolationSQLState = "23505"

// uniqueViolationConstraint は、errが一意制約の違反であれば、違反した制約の名前を返す。
// それ以外のエラーには空文字列を返す。
func uniqueViolationConstraint(err error) string {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolationSQLState {
		return ""
	}

	return pgErr.ConstraintName
}
