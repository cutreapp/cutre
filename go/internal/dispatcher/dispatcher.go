// Package dispatcher はジョブキュー (River) へのジョブの投入と、ジョブの引数を定義する。
//
// Args型はジョブを投入する側 (UseCase・定期ジョブの登録) と、処理する側 (internal/worker) の両方が参照する。
package dispatcher

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"

	"github.com/cutreapp/cutre/go/internal/model"
)

// cleanupMaxAttempts は定期的な削除のジョブを試行する回数の上限。
// 失敗しても次の定期実行で同じ行が対象になるため、何度も粘らずに次の回へ任せる。
const cleanupMaxAttempts = 3

// DeleteExpiredUserSessionsArgs は期限切れのセッションを削除するジョブの引数。
// 対象は実行した時点の時刻で決まるため、引数を持たない。
type DeleteExpiredUserSessionsArgs struct{}

// Kind はジョブの種類を返す。
func (DeleteExpiredUserSessionsArgs) Kind() string { return "delete_expired_user_sessions" }

// InsertOpts はジョブのInsertオプションを返す。
func (DeleteExpiredUserSessionsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: cleanupMaxAttempts}
}

// DeleteExpiredRateLimitsArgs は保持期間を過ぎたレート制限のカウンターを削除するジョブの引数。
// 保持期間はUseCaseが持つため、引数を持たない。
type DeleteExpiredRateLimitsArgs struct{}

// Kind はジョブの種類を返す。
func (DeleteExpiredRateLimitsArgs) Kind() string { return "delete_expired_rate_limits" }

// InsertOpts はジョブのInsertオプションを返す。
func (DeleteExpiredRateLimitsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: cleanupMaxAttempts}
}

// DeleteExpiredEmailConfirmationsArgs は使われずに残ったメールアドレスの確認を削除するジョブの引数。
// 保持期間はUseCaseが持つため、引数を持たない。
type DeleteExpiredEmailConfirmationsArgs struct{}

// Kind はジョブの種類を返す。
func (DeleteExpiredEmailConfirmationsArgs) Kind() string { return "delete_expired_email_confirmations" }

// InsertOpts はジョブのInsertオプションを返す。
func (DeleteExpiredEmailConfirmationsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: cleanupMaxAttempts}
}

// emailMaxAttempts はメールを送るジョブを試行する回数の上限。
// 送信の失敗は一時的なもの (ResendのAPIの障害など) が多く、確認コードの有効期限 (15分) の中で何度か再試行させる。
const emailMaxAttempts = 5

// SendEmailConfirmationArgs は確認コードのメールを送るジョブの引数。
type SendEmailConfirmationArgs struct {
	Email  string       `json:"email"`
	Code   string       `json:"code"`
	Locale model.Locale `json:"locale"`
}

// Kind はジョブの種類を返す。
func (SendEmailConfirmationArgs) Kind() string { return "send_email_confirmation" }

// InsertOpts はジョブのInsertオプションを返す。
func (SendEmailConfirmationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: emailMaxAttempts}
}

// SendAlreadyRegisteredNoticeArgs は、登録済みのメールアドレスで登録を始めた人へ、ログインを案内するメールを送るジョブの引数。
type SendAlreadyRegisteredNoticeArgs struct {
	Email  string       `json:"email"`
	Locale model.Locale `json:"locale"`
}

// Kind はジョブの種類を返す。
func (SendAlreadyRegisteredNoticeArgs) Kind() string { return "send_already_registered_notice" }

// InsertOpts はジョブのInsertオプションを返す。
func (SendAlreadyRegisteredNoticeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: emailMaxAttempts}
}

// SendPasswordResetArgs はパスワードリセットのメールを送るジョブの引数。
//
// リンクのトークンは引数に持たせず、ジョブを処理するときに作る。
// ジョブの行もデータベースに残るため、平文のトークンを載せると、ダイジェストだけを保存する意味が無くなる。
// 宛先のメールアドレスも持たせず、処理するときにユーザーから引く。
type SendPasswordResetArgs struct {
	UserID model.UserID `json:"user_id"`
	Locale model.Locale `json:"locale"`
}

// Kind はジョブの種類を返す。
func (SendPasswordResetArgs) Kind() string { return "send_password_reset" }

// InsertOpts はジョブのInsertオプションを返す。
func (SendPasswordResetArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: river.QueueDefault, MaxAttempts: emailMaxAttempts}
}

// Dispatcher はジョブをキューへ投入する。
//
// 投入だけを行うRiverのクライアントを、アプリケーションと同じ database/sql の接続の上に作る。
// ジョブを処理するクライアント (internal/worker) とは別に持つのは、投入をRepositoryと同じトランザクションに
// 参加させるため。ジョブの行と、ジョブが指すデータの行を一緒にコミット・ロールバックできる。
type Dispatcher struct {
	client *river.Client[*sql.Tx]
	tx     *sql.Tx
}

// NewDispatcher は Dispatcher を生成する。
func NewDispatcher(db *sql.DB) (*Dispatcher, error) {
	// QueuesとWorkersを持たないクライアントは投入だけを行い、ジョブを処理しない。
	client, err := river.NewClient(riverdatabasesql.New(db), &river.Config{})
	if err != nil {
		return nil, fmt.Errorf("ジョブを投入するクライアントの作成に失敗: %w", err)
	}

	return &Dispatcher{client: client}, nil
}

// WithTx はジョブをtx内で投入する新しい Dispatcher を返す。
func (d *Dispatcher) WithTx(tx *sql.Tx) *Dispatcher {
	return &Dispatcher{client: d.client, tx: tx}
}

// EnqueueEmailConfirmation は確認コードのメールを送るジョブを投入する。
func (d *Dispatcher) EnqueueEmailConfirmation(ctx context.Context, email, code string, locale model.Locale) error {
	return d.insert(ctx, SendEmailConfirmationArgs{Email: email, Code: code, Locale: locale})
}

// EnqueueAlreadyRegisteredNotice は、登録済みのメールアドレスへログインを案内するメールを送るジョブを投入する。
func (d *Dispatcher) EnqueueAlreadyRegisteredNotice(ctx context.Context, email string, locale model.Locale) error {
	return d.insert(ctx, SendAlreadyRegisteredNoticeArgs{Email: email, Locale: locale})
}

// EnqueuePasswordReset はパスワードリセットのメールを送るジョブを投入する。
func (d *Dispatcher) EnqueuePasswordReset(ctx context.Context, userID model.UserID, locale model.Locale) error {
	return d.insert(ctx, SendPasswordResetArgs{UserID: userID, Locale: locale})
}

// insert はArgs型のInsertオプションを添えてジョブを投入する。
// nilを渡すとArgs型に書いた試行回数の上限が使われないため、明示して渡す。
func (d *Dispatcher) insert(ctx context.Context, args insertOptsArgs) error {
	opts := args.InsertOpts()

	var err error
	if d.tx != nil {
		_, err = d.client.InsertTx(ctx, d.tx, args, &opts)
	} else {
		_, err = d.client.Insert(ctx, args, &opts)
	}

	return err
}

// insertOptsArgs は自分のInsertオプションを持つジョブの引数。
type insertOptsArgs interface {
	river.JobArgs
	InsertOpts() river.InsertOpts
}
