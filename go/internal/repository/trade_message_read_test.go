package repository_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestTradeMessageReadRepository は、読んだ時刻を記録して引き、古い時刻では戻さないことと、交換とユーザーの組ごとに持つことを検証する。
func TestTradeMessageReadRepository(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeMessageReadRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	readAt := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

	if got, err := repo.FindLastReadPosition(t.Context(), tradeID, userID); err != nil || got != nil {
		t.Errorf("読んだことが無いとき: FindLastReadPosition() = (%v, %v)、(nil, nil) を期待", got, err)
	}

	olderID := testutil.NewTradeMessageBuilder(t, tx, tradeID, partnerID, "古いメッセージ").WithCreatedAt(readAt.Add(-time.Hour)).Build()
	firstID := testutil.NewTradeMessageBuilder(t, tx, tradeID, partnerID, "同時刻の1通目").WithCreatedAt(readAt).Build()
	secondID := testutil.NewTradeMessageBuilder(t, tx, tradeID, partnerID, "同時刻の2通目").WithCreatedAt(readAt).Build()
	if firstID.String() > secondID.String() {
		firstID, secondID = secondID, firstID
	}
	for _, position := range []struct {
		at time.Time
		id model.TradeMessageID
	}{
		{readAt.Add(-time.Hour), olderID}, {readAt, firstID}, {readAt, secondID}, {readAt, firstID},
	} {
		if err := repo.Save(t.Context(), tradeID, userID, model.TradeMessageReadPosition{CreatedAt: position.at, MessageID: position.id}); err != nil {
			t.Fatalf("Save(%v)のエラー = %v", position, err)
		}
	}
	if got, err := repo.FindLastReadPosition(t.Context(), tradeID, userID); err != nil || got == nil || !got.CreatedAt.Equal(readAt) || got.MessageID != secondID {
		t.Errorf("FindLastReadPosition() = (%v, %v)、古い位置で戻さず (%v, %s) を期待", got, err, readAt, secondID)
	}
	if got, err := repo.FindLastReadPosition(t.Context(), tradeID, partnerID); err != nil || got != nil {
		t.Errorf("相手: FindLastReadPosition() = (%v, %v)、(nil, nil) を期待", got, err)
	}
}

// TestTradeMessageReadRepository_SameTimestamp は、同じ送信時刻でも読んだIDより後に並ぶメッセージを未読に残すことを検証する。
func TestTradeMessageReadRepository_SameTimestamp(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	firstID := testutil.NewTradeMessageBuilder(t, tx, tradeID, partnerID, "同時刻の1通目").WithCreatedAt(at).Build()
	secondID := testutil.NewTradeMessageBuilder(t, tx, tradeID, partnerID, "同時刻の2通目").WithCreatedAt(at).Build()
	if firstID.String() > secondID.String() {
		firstID, secondID = secondID, firstID
	}

	readRepo := repository.NewTradeMessageReadRepository(db).WithTx(tx)
	if err := readRepo.Save(t.Context(), tradeID, userID, model.TradeMessageReadPosition{CreatedAt: at, MessageID: firstID}); err != nil {
		t.Fatalf("Save()のエラー = %v", err)
	}
	messageRepo := repository.NewTradeMessageRepository(db).WithTx(tx)
	counts, err := messageRepo.CountUnreadByTradeIDs(t.Context(), userID, []model.TradeID{tradeID})
	if err != nil || counts[tradeID] != 1 {
		t.Errorf("CountUnreadByTradeIDs() = (%v, %v)、同時刻の後の1通を期待", counts, err)
	}
	count, err := messageRepo.CountUnreadByUserID(t.Context(), userID)
	if err != nil || count != 1 {
		t.Errorf("CountUnreadByUserID() = (%d, %v)、同時刻の後の1通を期待", count, err)
	}
}

// TestTradeMessageReadRepository_OtherTradeMessage は、別の交換のメッセージを読んだ位置として記録できないことを検証する。
// 読んだ位置の比較に別の交換の時刻とIDが紛れ込まないよう、複合外部キーで拒否する。
func TestTradeMessageReadRepository_OtherTradeMessage(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	otherTradeID := testutil.NewTradeBuilder(t, tx, partnerID, userID).Build()
	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	otherMessageID := testutil.NewTradeMessageBuilder(t, tx, otherTradeID, partnerID, "別の交換のメッセージ").WithCreatedAt(at).Build()

	err := repository.NewTradeMessageReadRepository(db).WithTx(tx).Save(t.Context(), tradeID, userID, model.TradeMessageReadPosition{CreatedAt: at, MessageID: otherMessageID})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "trade_message_reads_trade_id_last_read_message_id_fkey" {
		t.Errorf("Save()のエラー = %v、複合外部キーの違反を期待", err)
	}
}
