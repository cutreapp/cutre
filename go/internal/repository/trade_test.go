package repository_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestTradeRepository_Create は、申し込んだ人から申し込まれた人への、返事待ちの交換を作ることを検証する。
func TestTradeRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()

	trade, err := repo.Create(context.Background(), proposerID, receiverID)
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if trade.ProposerUserID != proposerID || trade.ReceiverUserID != receiverID || trade.Status != model.TradeStatusPending ||
		trade.MatchedAt != nil || trade.EndedAt != nil || trade.ProposerCompletedAt != nil || trade.ReceiverCompletedAt != nil {
		t.Errorf("作った交換 = %+v、入力の2人の返事待ちの交換を期待", trade)
	}
}

// TestTradeRepository_ExistsInProgressByUserID は、申し込んだか申し込まれた交換のうち、返事待ちとマッチ成立だけを進行中と数えることを検証する。
func TestTradeRepository_ExistsInProgressByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)

	tests := []struct {
		name     string
		status   model.TradeStatus
		proposer bool
		want     bool
	}{
		{name: "申し込んだ返事待ち", status: model.TradeStatusPending, proposer: true, want: true},
		{name: "申し込まれた返事待ち", status: model.TradeStatusPending, want: true},
		{name: "申し込まれたマッチ成立", status: model.TradeStatusMatched, want: true},
		{name: "取り下げ", status: model.TradeStatusWithdrawn, proposer: true},
		{name: "お断り", status: model.TradeStatusDeclined},
		{name: "交換できた", status: model.TradeStatusCompleted, proposer: true},
		{name: "交換できなかった", status: model.TradeStatusFailed},
		{name: "やめた", status: model.TradeStatusCancelled, proposer: true},
	}
	for _, tt := range tests {
		userID := testutil.NewUserBuilder(t, tx).Build()
		partnerID := testutil.NewUserBuilder(t, tx).Build()
		if tt.proposer {
			testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(tt.status).Build()
		} else {
			testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(tt.status).Build()
		}

		got, err := repo.ExistsInProgressByUserID(context.Background(), userID)
		if err != nil || got != tt.want {
			t.Errorf("%s: ExistsInProgressByUserID() = (%v, %v)、(%v, nil) を期待", tt.name, got, err, tt.want)
		}
	}

	if got, err := repo.ExistsInProgressByUserID(context.Background(), testutil.NewUserBuilder(t, tx).Build()); err != nil || got {
		t.Errorf("交換の無いユーザー: ExistsInProgressByUserID() = (%v, %v)、(false, nil) を期待", got, err)
	}
}

// TestTradeRepository_CountInProgressByUserID は、ユーザーが申し込んだか申し込まれた交換のうち、
// 返事待ちとマッチ成立だけを数えることを検証する。
func TestTradeRepository_CountInProgressByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusMatched).Build()
	for _, status := range []model.TradeStatus{model.TradeStatusWithdrawn, model.TradeStatusDeclined, model.TradeStatusCompleted, model.TradeStatusFailed, model.TradeStatusCancelled} {
		testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(status).Build()
	}
	// ほかの2人の交換は数えない。
	testutil.NewTradeBuilder(t, tx, partnerID, testutil.NewUserBuilder(t, tx).Build()).Build()

	if got, err := repo.CountInProgressByUserID(context.Background(), userID); err != nil || got != 2 {
		t.Errorf("CountInProgressByUserID() = (%d, %v)、(2, nil) を期待", got, err)
	}
	if got, err := repo.CountInProgressByUserID(context.Background(), testutil.NewUserBuilder(t, tx).Build()); err != nil || got != 0 {
		t.Errorf("交換の無いユーザー: CountInProgressByUserID() = (%d, %v)、(0, nil) を期待", got, err)
	}
}

// TestTradeItemRepository_CreateMany は、交換の品としてアイテムを1点ずつ入れ、空のときは何もしないことを検証する。
func TestTradeItemRepository_CreateMany(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeItemRepository(db).WithTx(tx)
	ctx := context.Background()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()
	itemIDs := []model.ItemID{testutil.NewItemBuilder(t, tx, proposerID, goodsID).Build(), testutil.NewItemBuilder(t, tx, receiverID, goodsID).Build()}

	if err := repo.CreateMany(ctx, tradeID, nil); err != nil {
		t.Fatalf("CreateMany(空)のエラー = %v", err)
	}
	if err := repo.CreateMany(ctx, tradeID, itemIDs); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM trade_items WHERE trade_id = $1 AND item_id = ANY($2::uuid[])",
		uuid.UUID(tradeID), "{"+itemIDs[0].String()+","+itemIDs[1].String()+"}").Scan(&count); err != nil || count != 2 {
		t.Errorf("交換の品の数 = (%d, %v)、2を期待", count, err)
	}
}

// TestTradeEventRepository_Create は、交換の出来事を、起こしたユーザー・種類・理由とともに記録することを検証する。
func TestTradeEventRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeEventRepository(db).WithTx(tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, testutil.NewUserBuilder(t, tx).Build()).Build()

	event, err := repo.Create(context.Background(), tradeID, proposerID, model.TradeEventKindProposed, nil)
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if event.TradeID != tradeID || event.ActorUserID != proposerID || event.Kind != model.TradeEventKindProposed || event.Reason != nil {
		t.Errorf("記録した出来事 = %+v、申し込んだ人の理由の無い申し込みを期待", event)
	}
}

// TestTradeMessageRepository_Create は、交換のメッセージを、送った人と本文とともに取り消していない状態で記録することを検証する。
func TestTradeMessageRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeMessageRepository(db).WithTx(tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, testutil.NewUserBuilder(t, tx).Build()).Build()

	message, err := repo.Create(context.Background(), tradeID, proposerID, "はじめまして")
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if message.TradeID != tradeID || message.SenderUserID != proposerID || message.Body != "はじめまして" || message.RetractedAt != nil {
		t.Errorf("記録したメッセージ = %+v、申し込んだ人の取り消していないメッセージを期待", message)
	}
}

// TestTradeRepository_FindByID は、交換をIDで引き、無いときは (nil, nil) を返すことを検証する。
func TestTradeRepository_FindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()

	trade, err := repo.FindByID(context.Background(), tradeID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if trade == nil || trade.ID != tradeID || trade.ProposerUserID != proposerID || trade.ReceiverUserID != receiverID || trade.Status != model.TradeStatusMatched {
		t.Errorf("引いた交換 = %+v、作ったマッチ成立の交換を期待", trade)
	}

	if got, err := repo.FindByID(context.Background(), model.TradeID(uuid.New())); err != nil || got != nil {
		t.Errorf("無い交換: FindByID() = (%+v, %v)、(nil, nil) を期待", got, err)
	}
}

// TestTradeRepository_ListInProgressByUserID は、申し込んだか申し込まれた進行中の交換だけを、新しく申し込まれた順に返すことを検証する。
func TestTradeRepository_ListInProgressByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	proposed := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	received := testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusMatched).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusWithdrawn).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusCompleted).Build()
	// ほかの2人の交換は含めない。
	testutil.NewTradeBuilder(t, tx, partnerID, testutil.NewUserBuilder(t, tx).Build()).Build()

	trades, err := repo.ListInProgressByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListInProgressByUserID()のエラー = %v", err)
	}
	// 同じトランザクションで作った行は created_at が同じため、UUIDv7のidの新しい順に並ぶ。
	if len(trades) != 2 || trades[0].ID != received || trades[1].ID != proposed {
		t.Errorf("進行中の交換 = %+v、[申し込まれたマッチ成立, 申し込んだ返事待ち] を期待", trades)
	}
}

// TestTradeRepository_ListEndedByUserID は、ユーザーが申し込んだか申し込まれた、終わった交換だけを、終わった時刻が新しい順に返すことを検証する。
func TestTradeRepository_ListEndedByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	now := time.Now()
	completed := testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusCompleted).WithEndedAt(now.Add(-2 * time.Hour)).Build()
	withdrawn := testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusWithdrawn).WithEndedAt(now.Add(-time.Hour)).Build()
	cancelled := testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusCancelled).WithEndedAt(now.Add(-3 * time.Hour)).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusMatched).Build()
	// ほかの2人の交換は含めない。
	testutil.NewTradeBuilder(t, tx, partnerID, testutil.NewUserBuilder(t, tx).Build()).WithStatus(model.TradeStatusFailed).Build()

	trades, err := repo.ListEndedByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListEndedByUserID()のエラー = %v", err)
	}
	if len(trades) != 3 || trades[0].ID != withdrawn || trades[1].ID != completed || trades[2].ID != cancelled {
		t.Errorf("終わった交換 = %+v、[取り下げ, 交換できた, やめた] を期待", trades)
	}
}

// TestTradeRepository_CountEndedByUserIDGroupByStatus は、ユーザーの終わった交換の数を段階ごとに返し、進行中の交換とほかの2人の交換を数えないことを検証する。
func TestTradeRepository_CountEndedByUserIDGroupByStatus(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusDeclined).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusMatched).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, testutil.NewUserBuilder(t, tx).Build()).WithStatus(model.TradeStatusCompleted).Build()

	counts, err := repo.CountEndedByUserIDGroupByStatus(context.Background(), userID)
	if err != nil {
		t.Fatalf("CountEndedByUserIDGroupByStatus()のエラー = %v", err)
	}
	want := map[model.TradeStatus]int64{model.TradeStatusCompleted: 2, model.TradeStatusDeclined: 1}
	if len(counts) != len(want) || counts[model.TradeStatusCompleted] != 2 || counts[model.TradeStatusDeclined] != 1 {
		t.Errorf("段階ごとの数 = %v、%v を期待", counts, want)
	}

	counts, err = repo.CountEndedByUserIDGroupByStatus(context.Background(), testutil.NewUserBuilder(t, tx).Build())
	if err != nil || len(counts) != 0 {
		t.Errorf("交換の無いユーザー: (%v, %v)、空を期待", counts, err)
	}
}

// TestTradeRepository_ListByUserIDOrderByLatestMessage は、ユーザーの交換を終わったものも含めて、最新のメッセージが新しい順に返し、
// メッセージの無い交換は申し込まれた時刻で並べることを検証する。
func TestTradeRepository_ListByUserIDOrderByLatestMessage(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	now := time.Now()
	older := testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusCompleted).Build()
	newer := testutil.NewTradeBuilder(t, tx, partnerID, userID).Build()
	// メッセージの無い交換は、申し込まれた時刻 (このトランザクションの時刻) で並ぶ。
	withoutMessage := testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	testutil.NewTradeMessageBuilder(t, tx, older, userID, "古いほうの交換の新しいメッセージ").WithCreatedAt(now.Add(time.Hour)).Build()
	testutil.NewTradeMessageBuilder(t, tx, newer, partnerID, "新しいほうの交換の古いメッセージ").WithCreatedAt(now.Add(-time.Hour)).Build()
	// ほかの2人の交換は含めない。
	testutil.NewTradeBuilder(t, tx, partnerID, testutil.NewUserBuilder(t, tx).Build()).Build()

	trades, err := repo.ListByUserIDOrderByLatestMessage(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListByUserIDOrderByLatestMessage()のエラー = %v", err)
	}
	if len(trades) != 3 || trades[0].ID != older || trades[1].ID != withoutMessage || trades[2].ID != newer {
		t.Errorf("交換 = %+v、[最新のメッセージが1時間後, メッセージ無し, 最新のメッセージが1時間前] を期待", trades)
	}
}

// TestTradeRepository_CountAwaitingByUserID は、ユーザーの返事や確認を待っている交換だけを数えることを検証する。
func TestTradeRepository_CountAwaitingByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	now := time.Now()
	// 数えるもの: 申し込まれた返事待ちと、相手だけが「交換できた」を押したマッチ成立 (申し込んだ側・申し込まれた側)。
	testutil.NewTradeBuilder(t, tx, partnerID, userID).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusMatched).WithReceiverCompletedAt(now).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusMatched).WithProposerCompletedAt(now).Build()
	// 数えないもの: 申し込んだ返事待ち・どちらも押していないマッチ成立・自分だけが押したマッチ成立・終わった交換。
	testutil.NewTradeBuilder(t, tx, userID, partnerID).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusMatched).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusMatched).WithProposerCompletedAt(now).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusDeclined).Build()

	if got, err := repo.CountAwaitingByUserID(context.Background(), userID); err != nil || got != 3 {
		t.Errorf("CountAwaitingByUserID() = (%d, %v)、(3, nil) を期待", got, err)
	}
}

// TestTradeRepository_Withdraw は、申し込んだ人の返事待ちの交換だけを取り下げ、終わった日時を記録することを検証する。
func TestTradeRepository_Withdraw(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	ctx := context.Background()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	pending := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	matched := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()

	if withdrawn, err := repo.Withdraw(ctx, pending, receiverID); err != nil || withdrawn {
		t.Errorf("申し込まれた人: Withdraw() = (%v, %v)、(false, nil) を期待", withdrawn, err)
	}
	if withdrawn, err := repo.Withdraw(ctx, matched, proposerID); err != nil || withdrawn {
		t.Errorf("マッチ成立: Withdraw() = (%v, %v)、(false, nil) を期待", withdrawn, err)
	}
	if withdrawn, err := repo.Withdraw(ctx, pending, proposerID); err != nil || !withdrawn {
		t.Fatalf("申し込んだ人の返事待ち: Withdraw() = (%v, %v)、(true, nil) を期待", withdrawn, err)
	}

	trade, err := repo.FindByID(ctx, pending)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if trade.Status != model.TradeStatusWithdrawn || trade.EndedAt == nil {
		t.Errorf("取り下げた交換 = %+v、終わった日時のある取り下げを期待", trade)
	}
	if withdrawn, err := repo.Withdraw(ctx, pending, proposerID); err != nil || withdrawn {
		t.Errorf("取り下げ済み: Withdraw() = (%v, %v)、(false, nil) を期待", withdrawn, err)
	}
}

// TestTradeRepository_Approve は、申し込まれた人の返事待ちの交換だけを承認し、マッチ成立にして承認した日時を記録することを検証する。
func TestTradeRepository_Approve(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	ctx := context.Background()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	pending := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	withdrawn := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusWithdrawn).Build()

	if approved, err := repo.Approve(ctx, pending, proposerID); err != nil || approved {
		t.Errorf("申し込んだ人: Approve() = (%v, %v)、(false, nil) を期待", approved, err)
	}
	if approved, err := repo.Approve(ctx, withdrawn, receiverID); err != nil || approved {
		t.Errorf("取り下げ: Approve() = (%v, %v)、(false, nil) を期待", approved, err)
	}
	if approved, err := repo.Approve(ctx, pending, receiverID); err != nil || !approved {
		t.Fatalf("申し込まれた人の返事待ち: Approve() = (%v, %v)、(true, nil) を期待", approved, err)
	}

	trade, err := repo.FindByID(ctx, pending)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if trade.Status != model.TradeStatusMatched || trade.MatchedAt == nil || trade.EndedAt != nil {
		t.Errorf("承認した交換 = %+v、承認した日時のある、終わっていないマッチ成立を期待", trade)
	}
	if approved, err := repo.Approve(ctx, pending, receiverID); err != nil || approved {
		t.Errorf("承認済み: Approve() = (%v, %v)、(false, nil) を期待", approved, err)
	}
}

// TestTradeRepository_Decline は、申し込まれた人の返事待ちの交換だけをお断りし、終わった日時を記録することを検証する。
func TestTradeRepository_Decline(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	ctx := context.Background()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	pending := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	matched := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()

	if declined, err := repo.Decline(ctx, pending, proposerID); err != nil || declined {
		t.Errorf("申し込んだ人: Decline() = (%v, %v)、(false, nil) を期待", declined, err)
	}
	if declined, err := repo.Decline(ctx, matched, receiverID); err != nil || declined {
		t.Errorf("マッチ成立: Decline() = (%v, %v)、(false, nil) を期待", declined, err)
	}
	if declined, err := repo.Decline(ctx, pending, receiverID); err != nil || !declined {
		t.Fatalf("申し込まれた人の返事待ち: Decline() = (%v, %v)、(true, nil) を期待", declined, err)
	}

	trade, err := repo.FindByID(ctx, pending)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if trade.Status != model.TradeStatusDeclined || trade.EndedAt == nil || trade.MatchedAt != nil {
		t.Errorf("お断りした交換 = %+v、終わった日時のあるお断りを期待", trade)
	}
	if declined, err := repo.Decline(ctx, pending, receiverID); err != nil || declined {
		t.Errorf("お断り済み: Decline() = (%v, %v)、(false, nil) を期待", declined, err)
	}
}

// TestTradeRepository_Fail は、マッチ成立の交換を、交換の2人のどちらからでも、どちらかが「交換できた」を押したあとでも、
// 「交換できなかった」の段階で終え、マッチ成立でない交換と交換の2人以外では終えないことを検証する。
func TestTradeRepository_Fail(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	ctx := context.Background()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	pending := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	matched := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()
	halfCompleted := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusMatched).WithReceiverCompletedAt(time.Now()).Build()

	if failed, err := repo.Fail(ctx, pending, proposerID); err != nil || failed {
		t.Errorf("返事待ち: Fail() = (%v, %v)、(false, nil) を期待", failed, err)
	}
	if failed, err := repo.Fail(ctx, matched, testutil.NewUserBuilder(t, tx).Build()); err != nil || failed {
		t.Errorf("交換の2人以外: Fail() = (%v, %v)、(false, nil) を期待", failed, err)
	}
	if failed, err := repo.Fail(ctx, matched, proposerID); err != nil || !failed {
		t.Fatalf("申し込んだ人のマッチ成立: Fail() = (%v, %v)、(true, nil) を期待", failed, err)
	}
	if failed, err := repo.Fail(ctx, halfCompleted, proposerID); err != nil || !failed {
		t.Errorf("相手だけが「交換できた」を押したマッチ成立: Fail() = (%v, %v)、(true, nil) を期待", failed, err)
	}

	trade, err := repo.FindByID(ctx, matched)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if trade.Status != model.TradeStatusFailed || trade.EndedAt == nil {
		t.Errorf("記録した交換 = %+v、終わった日時のある「交換できなかった」を期待", trade)
	}
	if failed, err := repo.Fail(ctx, matched, receiverID); err != nil || failed {
		t.Errorf("記録済み: Fail() = (%v, %v)、(false, nil) を期待", failed, err)
	}
}

// TestTradeRepository_Cancel は、どちらも「交換できた」を押していないマッチ成立の交換を、交換の2人のどちらからでもやめ、
// どちらかが押した交換・マッチ成立でない交換・交換の2人以外ではやめないことを検証する。
func TestTradeRepository_Cancel(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeRepository(db).WithTx(tx)
	ctx := context.Background()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	pending := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	matched := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()
	halfCompleted := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).WithStatus(model.TradeStatusMatched).WithProposerCompletedAt(time.Now()).Build()

	if cancelled, err := repo.Cancel(ctx, pending, proposerID); err != nil || cancelled {
		t.Errorf("返事待ち: Cancel() = (%v, %v)、(false, nil) を期待", cancelled, err)
	}
	if cancelled, err := repo.Cancel(ctx, halfCompleted, receiverID); err != nil || cancelled {
		t.Errorf("相手が「交換できた」を押したマッチ成立: Cancel() = (%v, %v)、(false, nil) を期待", cancelled, err)
	}
	if cancelled, err := repo.Cancel(ctx, matched, testutil.NewUserBuilder(t, tx).Build()); err != nil || cancelled {
		t.Errorf("交換の2人以外: Cancel() = (%v, %v)、(false, nil) を期待", cancelled, err)
	}
	if cancelled, err := repo.Cancel(ctx, matched, receiverID); err != nil || !cancelled {
		t.Fatalf("申し込まれた人のマッチ成立: Cancel() = (%v, %v)、(true, nil) を期待", cancelled, err)
	}

	trade, err := repo.FindByID(ctx, matched)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if trade.Status != model.TradeStatusCancelled || trade.EndedAt == nil {
		t.Errorf("やめた交換 = %+v、終わった日時のある「やめた」を期待", trade)
	}
	if cancelled, err := repo.Cancel(ctx, matched, proposerID); err != nil || cancelled {
		t.Errorf("やめた交換: Cancel() = (%v, %v)、(false, nil) を期待", cancelled, err)
	}
}

// TestTradeItemRepository_ListItemIDsByTradeIDs は、交換ごとの交換の品のアイテムを、入れた順に返すことを検証する。
func TestTradeItemRepository_ListItemIDsByTradeIDs(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeItemRepository(db).WithTx(tx)
	ctx := context.Background()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()
	giveItemID := testutil.NewItemBuilder(t, tx, proposerID, goodsID).Build()
	receiveItemID := testutil.NewItemBuilder(t, tx, receiverID, goodsID).Build()
	first := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	second := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	if err := repo.CreateMany(ctx, first, []model.ItemID{receiveItemID, giveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	if err := repo.CreateMany(ctx, second, []model.ItemID{giveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}

	got, err := repo.ListItemIDsByTradeIDs(ctx, []model.TradeID{first, second})
	if err != nil {
		t.Fatalf("ListItemIDsByTradeIDs()のエラー = %v", err)
	}
	if len(got) != 2 || !slices.Equal(got[first], []model.ItemID{receiveItemID, giveItemID}) || !slices.Equal(got[second], []model.ItemID{giveItemID}) {
		t.Errorf("交換の品 = %v、交換ごとに入れた順のアイテムを期待", got)
	}

	if got, err := repo.ListItemIDsByTradeIDs(ctx, nil); err != nil || got != nil {
		t.Errorf("空: ListItemIDsByTradeIDs() = (%v, %v)、(nil, nil) を期待", got, err)
	}
}

// TestTradeEventRepository_ListByTradeID は、交換の出来事を、その交換のものだけ起きた順に返すことを検証する。
func TestTradeEventRepository_ListByTradeID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewTradeEventRepository(db).WithTx(tx)
	ctx := context.Background()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	otherTradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	proposed, err := repo.Create(ctx, tradeID, proposerID, model.TradeEventKindProposed, nil)
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	withdrawn, err := repo.Create(ctx, tradeID, proposerID, model.TradeEventKindWithdrawn, nil)
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := repo.Create(ctx, otherTradeID, proposerID, model.TradeEventKindProposed, nil); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	events, err := repo.ListByTradeID(ctx, tradeID)
	if err != nil {
		t.Fatalf("ListByTradeID()のエラー = %v", err)
	}
	if len(events) != 2 || events[0].ID != proposed.ID || events[1].ID != withdrawn.ID {
		t.Errorf("交換の出来事 = %+v、[申し込み, 取り下げ] を期待", events)
	}
}
