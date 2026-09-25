package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestRateLimitRepository_Increment は、同じキーと時間枠では数が積み上がり、
// キーか時間枠が違えば別々に数えることを検証する。
func TestRateLimitRepository_Increment(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewRateLimitRepository(db).WithTx(tx)

	key := "sign_in:ip:" + testutil.UniqueAtname()
	window := time.Now().UTC().Truncate(time.Hour)

	first, err := repo.Increment(ctx, key, window)
	if err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if first.Count != 1 {
		t.Errorf("1回目のCount = %d、期待値 = 1", first.Count)
	}
	if first.Key != key {
		t.Errorf("Key = %q、期待値 = %q", first.Key, key)
	}
	if !first.WindowStart.Equal(window) {
		t.Errorf("WindowStart = %v、期待値 = %v", first.WindowStart, window)
	}

	second, err := repo.Increment(ctx, key, window)
	if err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if second.Count != 2 {
		t.Errorf("2回目のCount = %d、期待値 = 2", second.Count)
	}
	if second.ID != first.ID {
		t.Errorf("ID = %s、期待値 = %s (同じ行を更新することを期待)", second.ID, first.ID)
	}

	nextWindow, err := repo.Increment(ctx, key, window.Add(time.Hour))
	if err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if nextWindow.Count != 1 {
		t.Errorf("次の時間枠のCount = %d、期待値 = 1", nextWindow.Count)
	}

	otherKey, err := repo.Increment(ctx, key+":other", window)
	if err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if otherKey.Count != 1 {
		t.Errorf("別のキーのCount = %d、期待値 = 1", otherKey.Count)
	}
}

// TestRateLimitRepository_DeleteOlderThan は、過ぎた時間枠だけが消えて、
// 今の時間枠のカウンターが残ることを検証する。
func TestRateLimitRepository_DeleteOlderThan(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewRateLimitRepository(db).WithTx(tx)

	key := "sign_in:ip:" + testutil.UniqueAtname()
	current := time.Now().UTC().Truncate(time.Hour)
	old := current.Add(-48 * time.Hour)

	if _, err := repo.Increment(ctx, key, old); err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if _, err := repo.Increment(ctx, key, current); err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}

	if err := repo.DeleteOlderThan(ctx, current); err != nil {
		t.Fatalf("DeleteOlderThan()のエラー = %v", err)
	}

	// 消えた時間枠へ数え直すと1から始まる。残った時間枠は積み上がったままになる。
	revived, err := repo.Increment(ctx, key, old)
	if err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if revived.Count != 1 {
		t.Errorf("消した時間枠のCount = %d、期待値 = 1", revived.Count)
	}

	kept, err := repo.Increment(ctx, key, current)
	if err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if kept.Count != 2 {
		t.Errorf("残した時間枠のCount = %d、期待値 = 2", kept.Count)
	}
}
