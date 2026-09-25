package ratelimit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// longWindow は数え上げを確かめるテストで使う時間枠の長さ。
// 呼び出しの間に時間枠の境界をまたぐと数え直しになり、時刻だけを理由に失敗するため、
// 境界に当たることが実質無い長さにする。
const longWindow = 365 * 24 * time.Hour

// newLimiter はテスト用トランザクションの上で動く Limiter を返す。
func newLimiter(t *testing.T) (*ratelimit.Limiter, string) {
	t.Helper()

	db, tx := testutil.SetupTx(t)

	// キーはテストごとに変える。同じキーを共有すると、並行するテストのUPSERTが
	// 相手のトランザクションの終了 (ロールバック) まで一意制約の確認で待たされるため。
	return ratelimit.NewLimiter(repository.NewRateLimitRepository(db).WithTx(tx)), "test:" + testutil.UniqueAtname()
}

// TestLimiter_Check は、上限までの試行が受け付けられ、上限を超えた試行が拒まれることを検証する。
func TestLimiter_Check(t *testing.T) {
	t.Parallel()

	limiter, key := newLimiter(t)
	ctx := context.Background()
	input := ratelimit.CheckInput{Key: key, Limit: 2, Window: longWindow}

	first, err := limiter.Check(ctx, input)
	if err != nil {
		t.Fatalf("Check()のエラー = %v", err)
	}
	if !first.Allowed {
		t.Error("1回目のAllowed = false、期待値 = true")
	}
	if first.Count != 1 {
		t.Errorf("1回目のCount = %d、期待値 = 1", first.Count)
	}
	if first.Remaining != 1 {
		t.Errorf("1回目のRemaining = %d、期待値 = 1", first.Remaining)
	}
	if !first.ResetAt.After(time.Now()) {
		t.Errorf("ResetAt = %v、未来の時刻を期待", first.ResetAt)
	}

	second, err := limiter.Check(ctx, input)
	if err != nil {
		t.Fatalf("Check()のエラー = %v", err)
	}
	if !second.Allowed {
		t.Error("2回目のAllowed = false、期待値 = true")
	}
	if second.Remaining != 0 {
		t.Errorf("2回目のRemaining = %d、期待値 = 0", second.Remaining)
	}

	third, err := limiter.Check(ctx, input)
	if err != nil {
		t.Fatalf("Check()のエラー = %v", err)
	}
	if third.Allowed {
		t.Error("3回目のAllowed = true、期待値 = false")
	}
	if third.Count != 3 {
		t.Errorf("3回目のCount = %d、期待値 = 3 (上限を超えた試行も数える)", third.Count)
	}
	if third.Remaining != 0 {
		t.Errorf("3回目のRemaining = %d、期待値 = 0", third.Remaining)
	}
}

// TestLimiter_Allow は、上限を超えた試行だけが ErrExceeded になることを検証する。
func TestLimiter_Allow(t *testing.T) {
	t.Parallel()

	limiter, key := newLimiter(t)
	ctx := context.Background()
	input := ratelimit.CheckInput{Key: key, Limit: 1, Window: longWindow}

	if err := limiter.Allow(ctx, input); err != nil {
		t.Fatalf("1回目のAllow()のエラー = %v", err)
	}

	err := limiter.Allow(ctx, input)
	if !errors.Is(err, ratelimit.ErrExceeded) {
		t.Errorf("2回目のAllow()のエラー = %v、期待値 = ErrExceeded", err)
	}
}

// TestLimiter_Check_InvalidInput は、判定の条件が揃っていない呼び出しを数える前に止めることを検証する。
func TestLimiter_Check_InvalidInput(t *testing.T) {
	t.Parallel()

	key := "test:" + testutil.UniqueAtname()
	ctx := context.Background()

	tests := []struct {
		name  string
		input ratelimit.CheckInput
	}{
		{name: "キーが空", input: ratelimit.CheckInput{Limit: 1, Window: time.Hour}},
		{name: "上限が0", input: ratelimit.CheckInput{Key: key, Window: time.Hour}},
		{name: "上限が負の値", input: ratelimit.CheckInput{Key: key, Limit: -1, Window: time.Hour}},
		{name: "時間枠が0", input: ratelimit.CheckInput{Key: key, Limit: 1}},
		{name: "時間枠が負の値", input: ratelimit.CheckInput{Key: key, Limit: 1, Window: -time.Hour}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, tx := testutil.SetupTx(t)
			limiter := ratelimit.NewLimiter(repository.NewRateLimitRepository(db).WithTx(tx))
			if _, err := limiter.Check(ctx, tt.input); err == nil {
				t.Error("エラーを期待したが、nilだった")
			}

			// 時間枠が不正な入力もあるため、枠を限定せず、そのキーへの書き込みが無いことを確認する。
			var rows int
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM rate_limits WHERE key = $1", tt.input.Key).Scan(&rows); err != nil {
				t.Fatalf("カウンターの行数の取得に失敗しました: %v", err)
			}
			if rows != 0 {
				t.Errorf("不正入力のキーに属する行数 = %d、期待値 = 0", rows)
			}
		})
	}
}

// TestLimiter_DeleteExpired は、保持期間より前の時間枠のカウンターが消えることを検証する。
func TestLimiter_DeleteExpired(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewRateLimitRepository(db).WithTx(tx)
	limiter := ratelimit.NewLimiter(repo)

	key := "test:" + testutil.UniqueAtname()
	old := time.Now().UTC().Truncate(time.Hour).Add(-48 * time.Hour)

	if _, err := repo.Increment(ctx, key, old); err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}

	if err := limiter.DeleteExpired(ctx, 24*time.Hour); err != nil {
		t.Fatalf("DeleteExpired()のエラー = %v", err)
	}

	revived, err := repo.Increment(ctx, key, old)
	if err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if revived.Count != 1 {
		t.Errorf("Count = %d、期待値 = 1 (消えていることを期待)", revived.Count)
	}
}

// TestLimiter_DeleteExpired_InvalidRetention は、保持期間が正の値でなければ削除せずにエラーを返すことを検証する。
// 受け付けると、現在時刻より前に始まった進行中の時間枠まで消えてしまう。
func TestLimiter_DeleteExpired_InvalidRetention(t *testing.T) {
	t.Parallel()

	limiter, key := newLimiter(t)
	ctx := context.Background()
	input := ratelimit.CheckInput{Key: key, Limit: 10, Window: longWindow}

	if _, err := limiter.Check(ctx, input); err != nil {
		t.Fatalf("Check()のエラー = %v", err)
	}

	for _, retention := range []time.Duration{0, -time.Hour} {
		if err := limiter.DeleteExpired(ctx, retention); err == nil {
			t.Errorf("DeleteExpired(%s)でエラーを期待したが、nilだった", retention)
		}
	}

	result, err := limiter.Check(ctx, input)
	if err != nil {
		t.Fatalf("Check()のエラー = %v", err)
	}
	if result.Count != 2 {
		t.Errorf("Count = %d、期待値 = 2 (進行中の時間枠が残っていることを期待)", result.Count)
	}
}

// TestKeys は、数える単位が混ざらないキーを組み立てることを検証する。
func TestKeys(t *testing.T) {
	t.Parallel()

	if got, want := ratelimit.IPKey("sign_in", "203.0.113.10"), "sign_in:ip:203.0.113.10"; got != want {
		t.Errorf("IPKey() = %q、期待値 = %q", got, want)
	}
	if got, want := ratelimit.EmailKey("sign_in", "user@example.com"), "sign_in:email:user@example.com"; got != want {
		t.Errorf("EmailKey() = %q、期待値 = %q", got, want)
	}
	if got, want := ratelimit.UserKey("sign_in_two_factor", "0190a4c2-0000-7000-8000-000000000000"), "sign_in_two_factor:user:0190a4c2-0000-7000-8000-000000000000"; got != want {
		t.Errorf("UserKey() = %q、期待値 = %q", got, want)
	}

	// 用途が違えばキーも違う。1つの上限を複数の操作で分け合わないようにする。
	if ratelimit.IPKey("sign_in", "203.0.113.10") == ratelimit.IPKey("password_reset", "203.0.113.10") {
		t.Error("用途の違うキーが同じ値になった")
	}

	// IPv6は /64 単位でまとめる。同じネットワークの中でアドレスを変えても同じカウンターになる。
	if got, want := ratelimit.IPKey("sign_in", "2001:db8:1:2:aaaa::1"), "sign_in:ip:2001:db8:1:2::/64"; got != want {
		t.Errorf("IPKey() = %q、期待値 = %q", got, want)
	}
	if ratelimit.IPKey("sign_in", "2001:db8:1:2::1") != ratelimit.IPKey("sign_in", "2001:db8:1:2:ffff::9") {
		t.Error("同じ /64 のIPv6アドレスが別のキーになった")
	}
	if ratelimit.IPKey("sign_in", "2001:db8:1:2::1") == ratelimit.IPKey("sign_in", "2001:db8:1:3::1") {
		t.Error("別の /64 のIPv6アドレスが同じキーになった")
	}

	// IPv6に埋め込まれたIPv4は、IPv4のまま数えるキーと揃える。
	if got, want := ratelimit.IPKey("sign_in", "::ffff:203.0.113.10"), "sign_in:ip:203.0.113.10"; got != want {
		t.Errorf("IPKey() = %q、期待値 = %q", got, want)
	}

	// メールアドレスは大文字小文字を区別しない。揃えないと書き換えるだけで数え直せてしまう。
	if got, want := ratelimit.EmailKey("sign_in", "User@Example.com"), ratelimit.EmailKey("sign_in", "user@example.com"); got != want {
		t.Errorf("EmailKey() = %q、期待値 = %q", got, want)
	}
}
