// Package ratelimit は固定ウィンドウのレート制限を提供する。
//
// 時間を一定の長さの枠に区切り、枠ごとに試行の回数を数える。
// 枠が変われば数え直すため、直前の枠でどれだけ試したかは次の枠の判定に持ち越さない。
// カウンターはデータベースに持たせ、アプリケーションのプロセスが複数あっても同じ数を見るようにする。
package ratelimit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
)

// ErrExceeded は試行が上限に達していることを表す。
//
// 利用者に見せる文面ではなくsentinel errorで返すのは、翻訳の解決を呼び出し元 (UseCase) に任せるため。
// このパッケージは制限の判定だけを担い、どう伝えるかは判定を求めた側が決める。
var ErrExceeded = errors.New("レート制限の上限に達しています")

// Limiter は試行を数えて上限との関係を判定する。
type Limiter struct {
	repo *repository.RateLimitRepository
}

// NewLimiter は Limiter を生成する。
func NewLimiter(repo *repository.RateLimitRepository) *Limiter {
	return &Limiter{repo: repo}
}

// WithTx はカウンターをtx内で更新する新しい Limiter を返す。
//
// トランザクションに参加させるかは呼び出し元が決める。
// 失敗した試行も数えたい場合 (ログインの失敗など) は、ロールバックで加算が消えないよう
// トランザクションの外で数える。
func (l *Limiter) WithTx(tx *sql.Tx) *Limiter {
	return &Limiter{repo: l.repo.WithTx(tx)}
}

// CheckInput はレート制限の判定に必要な条件。
type CheckInput struct {
	// Key は何を単位に数えるか (IPKey / EmailKey / UserKey で組み立てる)。
	Key string
	// Limit は1つの時間枠で許す試行の回数。
	Limit int
	// Window は時間枠の長さ。
	Window time.Duration
}

// CheckResult はレート制限の判定の結果。
type CheckResult struct {
	// Allowed はこの試行を受け付けてよいかどうか。
	Allowed bool
	// Count は今の時間枠で数えた試行の回数 (この試行を含む)。
	Count int
	// Remaining は今の時間枠で残っている試行の回数。
	Remaining int
	// ResetAt は今の時間枠が終わり、数え直しになる時刻。
	ResetAt time.Time
}

// Check は試行を1つ数えたうえで、その試行を受け付けてよいかを返す。
//
// 上限を超えた試行も数え、時間枠の中で行われた試行の総数を後から把握できるようにする。
func (l *Limiter) Check(ctx context.Context, input CheckInput) (*CheckResult, error) {
	if input.Key == "" {
		return nil, errors.New("レート制限のキーが指定されていません")
	}
	if input.Limit <= 0 {
		return nil, fmt.Errorf("レート制限の上限が正の値ではありません: %d", input.Limit)
	}
	if input.Window <= 0 {
		return nil, fmt.Errorf("レート制限の時間枠が正の値ではありません: %s", input.Window)
	}

	// 時間枠は現在時刻を枠の長さで切り捨てて決める。
	// 試行ごとに枠の起点がずれると、同じ相手の試行が別々の枠に散って数えられなくなる。
	now := time.Now().UTC()
	windowStart := startOfWindow(now, input.Window)

	rateLimit, err := l.repo.Increment(ctx, input.Key, windowStart)
	if err != nil {
		return nil, fmt.Errorf("レート制限のカウンターの更新に失敗: %w", err)
	}

	count := int(rateLimit.Count)

	return &CheckResult{
		Allowed:   count <= input.Limit,
		Count:     count,
		Remaining: max(input.Limit-count, 0),
		ResetAt:   windowStart.Add(input.Window),
	}, nil
}

// startOfWindow は時刻を固定ウィンドウの開始時刻へ切り捨てる。
func startOfWindow(now time.Time, window time.Duration) time.Time {
	return now.UTC().Truncate(window)
}

// Allow は試行を1つ数え、受け付けられないときに ErrExceeded を返す。
// 残りの回数や解除の時刻を使わない呼び出し元のための短い形。
func (l *Limiter) Allow(ctx context.Context, input CheckInput) error {
	result, err := l.Check(ctx, input)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return ErrExceeded
	}

	return nil
}

// DeleteExpired はretentionより前に始まった時間枠のカウンターを削除する。
// 過ぎた枠のカウンターは判定に使われないため、定期的に消してテーブルの肥大化を防ぐ。
//
// retentionには使っている時間枠のうち最長のもの以上を渡す。
// 短いと進行中の枠のカウンターまで消え、その枠の試行が数え直しになる。
func (l *Limiter) DeleteExpired(ctx context.Context, retention time.Duration) error {
	if retention <= 0 {
		return fmt.Errorf("レート制限のカウンターの保持期間が正の値ではありません: %s", retention)
	}

	return l.repo.DeleteOlderThan(ctx, time.Now().UTC().Add(-retention))
}

// ipv6PrefixLen はIPv6アドレスを数えるときにまとめるネットワークの長さ。
const ipv6PrefixLen = 64

// IPKey はIPアドレスを単位に数えるキーを返す。
//
// IPv6は /64 のネットワーク単位で数える。利用者には /64 以上がまとめて割り当てられるのが普通で、
// アドレス単位で数えると、その中のアドレスを変えるだけで新しいカウンターを得られてしまうため。
func IPKey(action, ip string) string {
	// IPv6に埋め込まれたIPv4アドレスは、同じ相手をIPv4で数えるカウンターと揃えるため展開する。
	if addr, err := netip.ParseAddr(ip); err == nil {
		addr = addr.Unmap().WithZone("")
		if addr.Is6() {
			ip = netip.PrefixFrom(addr, ipv6PrefixLen).Masked().String()
		} else {
			ip = addr.String()
		}
	}

	return fmt.Sprintf("%s:ip:%s", action, ip)
}

// EmailKey はメールアドレスを単位に数えるキーを返す。
//
// 小文字に揃えるのは、usersのメールアドレスが大文字小文字を区別しないため。
// 揃えないと、同じ宛先へ大文字混じりで送り直すだけで新しいカウンターを得られてしまう。
func EmailKey(action, email string) string {
	return fmt.Sprintf("%s:email:%s", action, strings.ToLower(email))
}

// UserKey はユーザーを単位に数えるキーを返す。userID はユーザーのIDの文字列表記。
//
// パスワードを確かめた後の二要素認証のように、試行の相手がIDで決まる操作で使う。
// IPアドレスを変えながら1つのアカウントへ試行を続ける総当たりを、アカウントの側で抑える。
func UserKey(action, userID string) string {
	return fmt.Sprintf("%s:user:%s", action, userID)
}
