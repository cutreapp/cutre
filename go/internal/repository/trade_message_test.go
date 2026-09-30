package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// waitForTradeLock は blockerPID の交換行ロックを、queryFragment を含むクエリが待つまで待機する。
func waitForTradeLock(t *testing.T, db *sql.DB, blockerPID int, queryFragment string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		err := db.QueryRowContext(t.Context(),
			`SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
				  AND query LIKE '%' || $2 || '%'
			)`, blockerPID, queryFragment,
		).Scan(&waiting)
		if err != nil {
			t.Fatalf("交換行のロック待ちの確認に失敗: %v", err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s が交換行のロックを待たなかった", queryFragment)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestTradeMessageRepository_CreateInProgress_ConcurrentEnd は、交換終了の更新が先に行をロックした場合、
// 送信が更新を待ってから終了状態を再判定し、メッセージを記録しないことを検証する。
func TestTradeMessageRepository_CreateInProgress_ConcurrentEnd(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	tradeID := testutil.NewTradeBuilder(t, db, proposerID, receiverID).Build()

	endTx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("交換終了のトランザクションの開始に失敗: %v", err)
	}
	defer func() { _ = endTx.Rollback() }()
	if _, err := endTx.ExecContext(t.Context(), `UPDATE trades SET status = 'withdrawn', ended_at = now() WHERE id = $1`, uuid.UUID(tradeID)); err != nil {
		t.Fatalf("交換終了の更新に失敗: %v", err)
	}
	var blockerPID int
	if err := endTx.QueryRowContext(t.Context(), `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatalf("交換終了の接続PIDの取得に失敗: %v", err)
	}

	type result struct {
		message *model.TradeMessage
		err     error
	}
	done := make(chan result, 1)
	go func() {
		message, err := repository.NewTradeMessageRepository(db).CreateInProgress(t.Context(), tradeID, receiverID, "送れない本文")
		done <- result{message: message, err: err}
	}()
	waitForTradeLock(t, db, blockerPID, "INSERT INTO trade_messages")
	if err := endTx.Commit(); err != nil {
		t.Fatalf("交換終了の確定に失敗: %v", err)
	}
	select {
	case got := <-done:
		if got.err != nil || got.message != nil {
			t.Errorf("終了後の送信 = (%+v, %v)、(nil, nil) を期待", got.message, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("交換終了後も送信が完了しなかった")
	}
}

// TestTradeMessageRepository_CreateInProgress_ConcurrentSend は、送信が先に共有ロックを取った場合、
// 交換終了の更新が送信の確定を待ち、メッセージが残ることを検証する。
func TestTradeMessageRepository_CreateInProgress_ConcurrentSend(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	tradeID := testutil.NewTradeBuilder(t, db, proposerID, receiverID).Build()

	sendTx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("送信のトランザクションの開始に失敗: %v", err)
	}
	defer func() { _ = sendTx.Rollback() }()
	message, err := repository.NewTradeMessageRepository(db).WithTx(sendTx).CreateInProgress(t.Context(), tradeID, receiverID, "先に送った本文")
	if err != nil || message == nil {
		t.Fatalf("交換終了前の送信 = (%+v, %v)、メッセージを期待", message, err)
	}
	var blockerPID int
	if err := sendTx.QueryRowContext(t.Context(), `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatalf("送信の接続PIDの取得に失敗: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := db.ExecContext(t.Context(), `UPDATE trades SET status = 'withdrawn', ended_at = now() WHERE id = $1`, uuid.UUID(tradeID))
		done <- err
	}()
	waitForTradeLock(t, db, blockerPID, "UPDATE trades SET status")
	if err := sendTx.Commit(); err != nil {
		t.Fatalf("送信の確定に失敗: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("交換終了の更新に失敗: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("送信の確定後も交換終了が完了しなかった")
	}
	messages, err := repository.NewTradeMessageRepository(db).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(messages) != 1 || messages[0].ID != message.ID {
		t.Errorf("終了した交換のメッセージ = (%+v, %v)、先に送った1通を期待", messages, err)
	}
}

// TestTradeMessageRepository_CreateInProgress は、返事待ちとマッチ成立の交換にだけメッセージを記録し、
// 終わった交換と無い交換には記録せずに (nil, nil) を返すことを検証する。
func TestTradeMessageRepository_CreateInProgress(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeMessageRepository(db).WithTx(tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()

	tests := []struct {
		status model.TradeStatus
		want   bool
	}{
		{status: model.TradeStatusPending, want: true},
		{status: model.TradeStatusMatched, want: true},
		{status: model.TradeStatusWithdrawn},
		{status: model.TradeStatusDeclined},
		{status: model.TradeStatusCompleted},
		{status: model.TradeStatusFailed},
		{status: model.TradeStatusCancelled},
	}
	for _, tt := range tests {
		tradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(tt.status).Build()

		message, err := repo.CreateInProgress(context.Background(), tradeID, receiverID, "よろしくお願いします")
		if err != nil {
			t.Fatalf("%s: CreateInProgress()のエラー = %v", tt.status, err)
		}
		if (message != nil) != tt.want {
			t.Errorf("%s: 記録したか = %v、期待値 = %v", tt.status, message != nil, tt.want)
			continue
		}
		if message != nil && (message.TradeID != tradeID || message.SenderUserID != receiverID || message.Body != "よろしくお願いします" || message.RetractedAt != nil) {
			t.Errorf("%s: 記録したメッセージ = %+v、入力の交換・送った人・本文を期待", tt.status, message)
		}
	}

	message, err := repo.CreateInProgress(context.Background(), model.TradeID(uuid.New()), receiverID, "本文")
	if err != nil || message != nil {
		t.Errorf("無い交換へのCreateInProgress() = (%+v, %v)、(nil, nil) を期待", message, err)
	}
}

// TestTradeMessageRepository_ListByTradeID は、交換のメッセージだけを、取り消したものも含めて送った順に返すことを検証する。
func TestTradeMessageRepository_ListByTradeID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeMessageRepository(db).WithTx(tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	otherTradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	base := time.Now().Add(-time.Hour)
	second := testutil.NewTradeMessageBuilder(t, tx, tradeID, receiverID, "2通目").WithCreatedAt(base.Add(time.Minute)).WithRetractedAt(base.Add(2 * time.Minute)).Build()
	first := testutil.NewTradeMessageBuilder(t, tx, tradeID, proposerID, "1通目").WithCreatedAt(base).Build()
	testutil.NewTradeMessageBuilder(t, tx, otherTradeID, proposerID, "ほかの交換").Build()

	messages, err := repo.ListByTradeID(context.Background(), tradeID)
	if err != nil {
		t.Fatalf("ListByTradeID()のエラー = %v", err)
	}
	if len(messages) != 2 || messages[0].ID != first || messages[1].ID != second {
		t.Fatalf("メッセージ = %+v、1通目・2通目の順を期待", messages)
	}
	if messages[1].RetractedAt == nil || messages[1].Body != "2通目" {
		t.Errorf("取り消したメッセージ = %+v、取り消した時刻と本文を残したまま返すことを期待", messages[1])
	}
}

// TestTradeMessageRepository_FindByID は、メッセージを引き、無ければ (nil, nil) を返すことを検証する。
func TestTradeMessageRepository_FindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeMessageRepository(db).WithTx(tx)
	senderID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, senderID, testutil.NewUserBuilder(t, tx).Build()).Build()
	messageID := testutil.NewTradeMessageBuilder(t, tx, tradeID, senderID, "よろしくお願いします").Build()

	message, err := repo.FindByID(t.Context(), messageID)
	if err != nil || message == nil || message.TradeID != tradeID || message.SenderUserID != senderID || message.Body != "よろしくお願いします" {
		t.Errorf("FindByID() = (%+v, %v)、送ったメッセージを期待", message, err)
	}
	if message, err := repo.FindByID(t.Context(), model.TradeMessageID(uuid.New())); err != nil || message != nil {
		t.Errorf("無いメッセージ: FindByID() = (%+v, %v)、(nil, nil) を期待", message, err)
	}
}

// TestTradeMessageRepository_ListLatestByTradeIDs は、交換ごとの最新のメッセージを、取り消したものも含めて返し、
// メッセージの無い交換は入れないことを検証する。
func TestTradeMessageRepository_ListLatestByTradeIDs(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeMessageRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	now := time.Now()
	first := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	second := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	empty := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	testutil.NewTradeMessageBuilder(t, tx, first, userID, "古い").WithCreatedAt(now.Add(-time.Minute)).Build()
	firstLatest := testutil.NewTradeMessageBuilder(t, tx, first, partnerID, "新しい").WithCreatedAt(now).Build()
	secondLatest := testutil.NewTradeMessageBuilder(t, tx, second, userID, "取り消した").WithRetractedAt(now).Build()

	latest, err := repo.ListLatestByTradeIDs(t.Context(), []model.TradeID{first, second, empty})
	if err != nil {
		t.Fatalf("ListLatestByTradeIDs()のエラー = %v", err)
	}
	if len(latest) != 2 || latest[first].ID != firstLatest || latest[second].ID != secondLatest || latest[second].RetractedAt == nil {
		t.Errorf("最新のメッセージ = %+v、2つの交換の最新の1通 (取り消したものを含む) を期待", latest)
	}
	if latest, err := repo.ListLatestByTradeIDs(t.Context(), nil); err != nil || latest != nil {
		t.Errorf("空: ListLatestByTradeIDs() = (%v, %v)、(nil, nil) を期待", latest, err)
	}
}

// TestTradeMessageRepository_CountUnread は、相手が送った取り消していないメッセージのうち、最後に読んだ時刻より後のものを、
// 交換ごとと、ユーザーの交換すべてで数えることを検証する。
func TestTradeMessageRepository_CountUnread(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeMessageRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	lastReadAt := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	read := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	neverRead := testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusCompleted).Build()
	allRead := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	readMessageID := testutil.NewTradeMessageBuilder(t, tx, read, partnerID, "既読").WithCreatedAt(lastReadAt).Build()
	if err := repository.NewTradeMessageReadRepository(db).WithTx(tx).Save(t.Context(), read, userID, model.TradeMessageReadPosition{CreatedAt: lastReadAt, MessageID: readMessageID}); err != nil {
		t.Fatalf("Save()のエラー = %v", err)
	}
	// 数えるもの: 読んだ時刻より後の相手のメッセージと、読んだことの無い交換の相手のメッセージ。
	testutil.NewTradeMessageBuilder(t, tx, read, partnerID, "未読").WithCreatedAt(lastReadAt.Add(time.Minute)).Build()
	testutil.NewTradeMessageBuilder(t, tx, neverRead, partnerID, "未読").WithCreatedAt(lastReadAt.Add(-time.Hour)).Build()
	testutil.NewTradeMessageBuilder(t, tx, neverRead, partnerID, "未読").WithCreatedAt(lastReadAt.Add(-time.Minute)).Build()
	// 数えないもの: 読んだ時刻までのもの・自分のもの・取り消したもの。
	testutil.NewTradeMessageBuilder(t, tx, read, userID, "自分").WithCreatedAt(lastReadAt.Add(time.Hour)).Build()
	testutil.NewTradeMessageBuilder(t, tx, read, partnerID, "取り消した").WithCreatedAt(lastReadAt.Add(time.Hour)).WithRetractedAt(lastReadAt.Add(2 * time.Hour)).Build()
	allReadMessageID := testutil.NewTradeMessageBuilder(t, tx, allRead, partnerID, "既読").WithCreatedAt(lastReadAt.Add(-time.Minute)).Build()
	if err := repository.NewTradeMessageReadRepository(db).WithTx(tx).Save(t.Context(), allRead, userID, model.TradeMessageReadPosition{CreatedAt: lastReadAt, MessageID: allReadMessageID}); err != nil {
		t.Fatalf("Save()のエラー = %v", err)
	}
	// ほかの2人の交換のメッセージは数えない。
	otherTrade := testutil.NewTradeBuilder(t, tx, partnerID, testutil.NewUserBuilder(t, tx).Build()).Build()
	testutil.NewTradeMessageBuilder(t, tx, otherTrade, partnerID, "ほかの交換").Build()

	counts, err := repo.CountUnreadByTradeIDs(t.Context(), userID, []model.TradeID{read, neverRead, allRead})
	if err != nil {
		t.Fatalf("CountUnreadByTradeIDs()のエラー = %v", err)
	}
	if len(counts) != 2 || counts[read] != 1 || counts[neverRead] != 2 {
		t.Errorf("交換ごとの未読の数 = %v、読んだ交換に1通・読んだことの無い交換に2通を期待", counts)
	}
	if got, err := repo.CountUnreadByUserID(t.Context(), userID); err != nil || got != 3 {
		t.Errorf("CountUnreadByUserID() = (%d, %v)、(3, nil) を期待", got, err)
	}
	if got, err := repo.CountUnreadByUserID(t.Context(), partnerID); err != nil || got != 1 {
		t.Errorf("相手: CountUnreadByUserID() = (%d, %v)、読んだことの無い交換の自分宛ての1通で (1, nil) を期待", got, err)
	}
}

// TestTradeMessageRepository_Retract は、送った人のメッセージだけを取り消し、本文を残して、取り消した時刻を最初の1回のまま保つことを検証する。
func TestTradeMessageRepository_Retract(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeMessageRepository(db).WithTx(tx)
	senderID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, senderID, partnerID).Build()
	messageID := testutil.NewTradeMessageBuilder(t, tx, tradeID, senderID, "取り消す本文").Build()

	if retracted, err := repo.Retract(t.Context(), messageID, partnerID); err != nil || retracted {
		t.Errorf("相手: Retract() = (%v, %v)、(false, nil) を期待", retracted, err)
	}
	if retracted, err := repo.Retract(t.Context(), messageID, senderID); err != nil || !retracted {
		t.Fatalf("送った人: Retract() = (%v, %v)、(true, nil) を期待", retracted, err)
	}

	message, err := repo.FindByID(t.Context(), messageID)
	if err != nil || message.RetractedAt == nil || message.Body != "取り消す本文" {
		t.Fatalf("取り消したメッセージ = (%+v, %v)、本文を残した取り消したメッセージを期待", message, err)
	}
	if retracted, err := repo.Retract(t.Context(), messageID, senderID); err != nil || retracted {
		t.Errorf("取り消し済み: Retract() = (%v, %v)、(false, nil) を期待", retracted, err)
	}
}
