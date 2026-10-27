// Package worker はバックグラウンドジョブのクライアント (River) のライフサイクルを管理し、
// ジョブの引数をUseCaseの入力へ変換するワーカーを持つ。
package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/email"
	"github.com/cutreapp/cutre/go/internal/logging"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/sentry"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// 定期的な削除のジョブを実行する間隔。
// レート制限のカウンターとメールアドレスの確認は試行のたびに増えるため、セッションより短い間隔で消す。
const (
	deleteExpiredUserSessionsInterval       = 24 * time.Hour
	deleteExpiredRateLimitsInterval         = time.Hour
	deleteExpiredEmailConfirmationsInterval = time.Hour
)

// ワーカーの接続プールとジョブの並行数の設定。値はAnnictにそろえる。
//
// プールはRiverの内部の処理 (ジョブの取得・完了の記録・LISTEN・リーダーの選出・定期ジョブの投入) が使う。
// ジョブの中のUseCaseはdatabase/sqlの接続 (internal/database の MaxOpenConns) を使うため、
// poolMaxConns はジョブの並行数に合わせて増やす必要が無い。
// 並行数の上限は、ジョブがHTTPの側の接続を使い切らないよう MaxOpenConns より十分小さくする。
// max_connections との関係は internal/database の MaxOpenConns のコメントを参照。
const (
	poolMaxConns        = 10
	poolMinConns        = 2
	poolMaxConnLifetime = 5 * time.Minute
	poolMaxConnIdleTime = 2 * time.Minute
	defaultMaxWorkers   = 10
)

// Client はRiverのクライアントと、それが使う接続プールをまとめて保持する。
// 起動と停止を1つの単位として扱うため。
type Client struct {
	riverClient *river.Client[pgx.Tx]
	pool        *pgxpool.Pool
}

// NewClient はRiverのクライアントを組み立てる。
//
// RiverのドライバーはpgxのプールだけをDBの接続として受け付けるため、database/sqlの接続とは別にプールを開く。
// ワーカーが呼ぶUseCaseのRepositoryは、HTTPの側と同じdbで組み立てる。
// ワーカーだけが使う依存はここで組み立て、serve.goの配線へ漏らさない。
func NewClient(ctx context.Context, cfg *config.Config, db *sql.DB) (*Client, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("ワーカー用の接続文字列の解析に失敗: %w", err)
	}
	poolConfig.MaxConns = poolMaxConns
	poolConfig.MinConns = poolMinConns
	poolConfig.MaxConnLifetime = poolMaxConnLifetime
	poolConfig.MaxConnIdleTime = poolMaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("ワーカー用の接続プールの作成に失敗: %w", err)
	}

	userSessionRepo := repository.NewUserSessionRepository(db)
	limiter := ratelimit.NewLimiter(repository.NewRateLimitRepository(db))
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	userRepo := repository.NewUserRepository(db)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(db)

	emailSender := newEmailSender(cfg)

	workers := river.NewWorkers()
	river.AddWorker(workers, NewSendEmailConfirmationWorker(usecase.NewSendEmailConfirmationUsecase(email.NewConfirmationSender(emailSender))))
	river.AddWorker(workers, NewSendAlreadyRegisteredNoticeWorker(usecase.NewSendAlreadyRegisteredNoticeUsecase(
		email.NewAlreadyRegisteredNoticeSender(emailSender, cfg.AppURL()),
	)))
	river.AddWorker(workers, NewSendPasswordResetWorker(usecase.NewSendPasswordResetUsecase(
		db, userRepo, passwordResetTokenRepo, email.NewPasswordResetSender(emailSender, cfg.AppURL()),
	)))
	river.AddWorker(workers, NewDeleteExpiredUserSessionsWorker(usecase.NewDeleteExpiredUserSessionsUsecase(userSessionRepo)))
	river.AddWorker(workers, NewDeleteExpiredRateLimitsWorker(usecase.NewDeleteExpiredRateLimitsUsecase(limiter)))
	river.AddWorker(workers, NewDeleteExpiredEmailConfirmationsWorker(usecase.NewDeleteExpiredEmailConfirmationsUsecase(emailConfirmationRepo)))

	// Riverの内部のサービスは、一時的な接続の失敗をErrorで出力して自力で回復する。
	// デフォルトのロガーに流すとSentryのイベントになってしまうため、同じ形式でSentryへのファンアウトだけを持たない基底のハンドラーを渡す。
	// ジョブの失敗はRiverWorkerMiddlewareが送るため、対処が要るものは失われない。
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: defaultMaxWorkers}},
		Workers:      workers,
		PeriodicJobs: periodicJobs(),
		Middleware:   []rivertype.Middleware{sentry.RiverWorkerMiddleware()},
		Logger:       slog.New(logging.NewBaseHandler(os.Stderr, cfg.LogFormat)),
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("ジョブキューのクライアントの作成に失敗: %w", err)
	}

	return &Client{riverClient: riverClient, pool: pool}, nil
}

// periodicJobs は定期的に実行するジョブを返す。
//
// RunOnStartを付けて、クライアントが起動するたびに一度実行する。
// Riverは次の実行時刻をクライアントの起動時点から数えるため、間隔より短い周期でデプロイすると、
// 付けない限り一度も実行されない。どの削除も何度実行しても結果が同じで、起動のたびに走らせても害が無い。
// 複数のプロセスが起動していても、定期ジョブを投入するのはリーダーに選ばれた1つだけになる。
func periodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		newPeriodicJob(deleteExpiredUserSessionsInterval, dispatcher.DeleteExpiredUserSessionsArgs{}),
		newPeriodicJob(deleteExpiredRateLimitsInterval, dispatcher.DeleteExpiredRateLimitsArgs{}),
		newPeriodicJob(deleteExpiredEmailConfirmationsInterval, dispatcher.DeleteExpiredEmailConfirmationsArgs{}),
	}
}

// insertOptsArgs は自分のInsertオプションを持つジョブの引数。
type insertOptsArgs interface {
	river.JobArgs
	InsertOpts() river.InsertOpts
}

// newPeriodicJob はargsをintervalごとに投入する定期ジョブを作る。
// Insertオプションを明示して返すのは、nilを返すとArgs型に書いた試行回数の上限が使われないため。
func newPeriodicJob(interval time.Duration, args insertOptsArgs) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			opts := args.InsertOpts()
			return args, &opts
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

// Start はジョブの取得と処理を始める。
func (c *Client) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Riverのクライアントを起動します")

	if err := c.riverClient.Start(ctx); err != nil {
		c.pool.Close()
		return err
	}

	return nil
}

// Stop は処理中のジョブの完了を待ってクライアントを止め、接続プールを閉じる。
func (c *Client) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "Riverのクライアントを停止します")

	defer c.pool.Close()

	return c.riverClient.Stop(ctx)
}

// Client は基盤のRiverのクライアントを返す。
// ジョブを投入する側 (Dispatcher) へ渡すために使う。
func (c *Client) Client() *river.Client[pgx.Tx] {
	return c.riverClient
}

// newEmailSender は設定に応じたメールの送信の手段を返す。
// APIキーを設定していればResendで送り、空ならログへ出力する。本番でAPIキーが空の場合は設定の読み込みが起動を止める。
// メールを送るのはワーカーだけのため、選択もここに置く。
func newEmailSender(cfg *config.Config) email.Sender {
	if cfg.ResendAPIKey == "" {
		return email.NewLogSender()
	}

	return email.NewResendSender(cfg.ResendAPIKey, cfg.EmailFrom)
}

// idempotencyKey はジョブからメールの送信のIdempotencyKeyを作る。
// 同じジョブの再試行では同じキーになり、Resendが二重の送信を1通にまとめる。
func idempotencyKey(job *rivertype.JobRow) string {
	return fmt.Sprintf("%s/%d", job.Kind, job.ID)
}
