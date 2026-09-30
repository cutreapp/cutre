package usecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newCompleteTradeUsecase は、自分でトランザクションを開く CompleteTradeUsecase をテスト用のデータベースで組み立てる。
func newCompleteTradeUsecase() *usecase.CompleteTradeUsecase {
	db := testutil.GetTestDB()

	return usecase.NewCompleteTradeUsecase(
		db,
		validator.NewTradeCompletionValidator(),
		repository.NewItemRepository(db),
		repository.NewMessageConsentRepository(db),
		repository.NewTradeRepository(db),
		repository.NewTradeEventRepository(db),
		repository.NewTradeMessageRepository(db),
	)
}

// completionFixture は、マッチ成立の交換と、2人のリストのアイテム。
//
// 申し込んだ人は譲れるアイテム proposerGive (2点) を渡し、申し込まれた人は譲れるアイテム receiverGive (1点) を渡す。
// 受け取る側のほしいリストには、それぞれ同じグッズのアイテム (receiverWant 1点・proposerWant 3点) がある。
type completionFixture struct {
	proposerID   model.UserID
	receiverID   model.UserID
	tradeID      model.TradeID
	proposerGive model.ItemID
	receiverGive model.ItemID
	proposerWant model.ItemID
	receiverWant model.ItemID
}

// newCompletionFixture は、2人とも同意したうえでのマッチ成立の交換を作る。
func newCompletionFixture(t *testing.T) completionFixture {
	t.Helper()

	db := testutil.GetTestDB()
	categoryID := testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()
	proposerGoods := testutil.NewGoodsBuilder(t, db, categoryID).Build()
	receiverGoods := testutil.NewGoodsBuilder(t, db, categoryID).Build()
	f := completionFixture{
		proposerID: testutil.NewUserBuilder(t, db).Build(),
		receiverID: testutil.NewUserBuilder(t, db).Build(),
	}
	testutil.NewMessageConsentBuilder(t, db, f.proposerID).Build()
	testutil.NewMessageConsentBuilder(t, db, f.receiverID).Build()
	f.proposerGive = testutil.NewItemBuilder(t, db, f.proposerID, proposerGoods).WithQuantity(2).Build()
	f.receiverGive = testutil.NewItemBuilder(t, db, f.receiverID, receiverGoods).Build()
	f.receiverWant = testutil.NewItemBuilder(t, db, f.receiverID, proposerGoods).WithKind(model.ItemKindWant).Build()
	f.proposerWant = testutil.NewItemBuilder(t, db, f.proposerID, receiverGoods).WithKind(model.ItemKindWant).WithQuantity(3).Build()
	f.tradeID = testutil.NewTradeBuilder(t, db, f.proposerID, f.receiverID).WithStatus(model.TradeStatusMatched).Build()
	if err := repository.NewTradeItemRepository(db).CreateMany(t.Context(), f.tradeID, []model.ItemID{f.proposerGive, f.receiverGive}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}

	return f
}

// assertItem は、アイテム id の数量と状態が期待どおりかを確かめる。
func assertItem(t *testing.T, name string, id model.ItemID, wantQuantity int32, wantStatus model.ItemStatus) {
	t.Helper()

	item, err := repository.NewItemRepository(testutil.GetTestDB()).FindByID(t.Context(), id)
	if err != nil || item == nil || item.Quantity != wantQuantity || item.Status != wantStatus {
		t.Errorf("%s = (%+v, %v)、数量 %d・状態 %q を期待", name, item, err, wantQuantity, wantStatus)
	}
}

// TestCompleteTradeUsecase_Execute は、片方が押すと押した時刻と出来事・ひとことを記録してマッチ成立のままにし、
// もう片方も押すと交換を終えて、2人のリストの数量を1点ずつ減らし、0点になったアイテムをリストから外すことを検証する。
func TestCompleteTradeUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newCompleteTradeUsecase()
	f := newCompletionFixture(t)

	output, err := uc.Execute(t.Context(), usecase.CompleteTradeInput{UserID: f.receiverID, TradeID: f.tradeID, Note: " 受け取りました。ありがとうございました。 "})
	if err != nil || output.Completed {
		t.Fatalf("申し込まれた人の Execute() = (%+v, %v)、相手を待つ結果を期待", output, err)
	}
	trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), f.tradeID)
	if err != nil || trade.Status != model.TradeStatusMatched || trade.ReceiverCompletedAt == nil || trade.ProposerCompletedAt != nil || trade.EndedAt != nil {
		t.Errorf("片方が押したあとの交換 = (%+v, %v)、申し込まれた人の時刻だけが入ったマッチ成立を期待", trade, err)
	}
	assertItem(t, "片方が押したあとの申し込んだ人の譲れるアイテム", f.proposerGive, 2, model.ItemStatusListed)

	output, err = uc.Execute(t.Context(), usecase.CompleteTradeInput{UserID: f.proposerID, TradeID: f.tradeID})
	if err != nil || !output.Completed {
		t.Fatalf("申し込んだ人の Execute() = (%+v, %v)、交換を終える結果を期待", output, err)
	}
	trade, err = repository.NewTradeRepository(db).FindByID(t.Context(), f.tradeID)
	if err != nil || trade.Status != model.TradeStatusCompleted || trade.ProposerCompletedAt == nil || trade.ReceiverCompletedAt == nil || trade.EndedAt == nil {
		t.Errorf("2人とも押したあとの交換 = (%+v, %v)、2人の時刻と終わった日時のある「交換できた」を期待", trade, err)
	}

	events, err := repository.NewTradeEventRepository(db).ListByTradeID(t.Context(), f.tradeID)
	if err != nil || len(events) != 2 || events[0].Kind != model.TradeEventKindCompleted || events[0].ActorUserID != f.receiverID ||
		events[1].Kind != model.TradeEventKindCompleted || events[1].ActorUserID != f.proposerID {
		t.Errorf("交換の出来事 = (%+v, %v)、申し込まれた人・申し込んだ人の順の「交換できた」の2件を期待", events, err)
	}
	messages, err := repository.NewTradeMessageRepository(db).ListByTradeID(t.Context(), f.tradeID)
	if err != nil || len(messages) != 1 || messages[0].SenderUserID != f.receiverID || messages[0].Body != "受け取りました。ありがとうございました。" {
		t.Errorf("交換のメッセージ = (%+v, %v)、申し込まれた人の前後の空白を除いたひとことの1通を期待", messages, err)
	}

	assertItem(t, "申し込んだ人の譲れるアイテム", f.proposerGive, 1, model.ItemStatusListed)
	assertItem(t, "申し込まれた人の譲れるアイテム", f.receiverGive, 0, model.ItemStatusRemoved)
	assertItem(t, "申し込まれた人のほしいアイテム", f.receiverWant, 0, model.ItemStatusRemoved)
	assertItem(t, "申し込んだ人のほしいアイテム", f.proposerWant, 2, model.ItemStatusListed)
}

// TestCompleteTradeUsecase_Execute_ItemsOutsideList は、2人そろって交換を終えるとき、リストから外したアイテムと、
// 受け取る人のほしいリストに無いグッズ・ほかの人のほしいアイテムは減らさないことを検証する。
func TestCompleteTradeUsecase_Execute_ItemsOutsideList(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	categoryID := testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()
	goodsID := testutil.NewGoodsBuilder(t, db, categoryID).Build()
	removedGoodsID := testutil.NewGoodsBuilder(t, db, categoryID).Build()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	give := testutil.NewItemBuilder(t, db, proposerID, goodsID).WithQuantity(2).Build()
	removedGive := testutil.NewItemBuilder(t, db, proposerID, removedGoodsID).WithRemoved().Build()
	removedWant := testutil.NewItemBuilder(t, db, receiverID, removedGoodsID).WithKind(model.ItemKindWant).WithRemoved().Build()
	othersWant := testutil.NewItemBuilder(t, db, testutil.NewUserBuilder(t, db).Build(), goodsID).WithKind(model.ItemKindWant).Build()
	tradeID := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).WithReceiverCompletedAt(time.Now()).Build()
	if err := repository.NewTradeItemRepository(db).CreateMany(t.Context(), tradeID, []model.ItemID{give, removedGive}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}

	output, err := newCompleteTradeUsecase().Execute(t.Context(), usecase.CompleteTradeInput{UserID: proposerID, TradeID: tradeID})
	if err != nil || !output.Completed {
		t.Fatalf("Execute() = (%+v, %v)、交換を終える結果を期待", output, err)
	}

	assertItem(t, "リストにある譲れるアイテム", give, 1, model.ItemStatusListed)
	assertItem(t, "リストから外した譲れるアイテム", removedGive, 1, model.ItemStatusRemoved)
	assertItem(t, "リストから外したほしいアイテム", removedWant, 1, model.ItemStatusRemoved)
	assertItem(t, "ほかの人のほしいアイテム", othersWant, 1, model.ItemStatusListed)
}

// TestCompleteTradeUsecase_Execute_Rejected は、「交換できた」を押せないときに交換を変えず、出来事もメッセージも記録しないことを検証する。
// マッチ成立でない交換とすでに押した交換では、入力の誤りや同意より先にそのことを返す。
func TestCompleteTradeUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newCompleteTradeUsecase()
	proposerID := testutil.NewUserBuilder(t, db).Build()
	receiverID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, receiverID).Build()
	noConsentID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewMessageConsentBuilder(t, db, noConsentID).WithWithdrawnAt(time.Now()).Build()
	matched := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).Build()
	pending := testutil.NewTradeBuilder(t, db, proposerID, receiverID).Build()
	cancelled := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusCancelled).Build()
	pressed := testutil.NewTradeBuilder(t, db, proposerID, receiverID).WithStatus(model.TradeStatusMatched).WithReceiverCompletedAt(time.Now()).Build()
	withNoConsent := testutil.NewTradeBuilder(t, db, proposerID, noConsentID).WithStatus(model.TradeStatusMatched).Build()

	tests := []struct {
		name     string
		input    usecase.CompleteTradeInput
		wantCode model.AppErrorCode
	}{
		{name: "無い交換", input: usecase.CompleteTradeInput{UserID: receiverID, TradeID: model.TradeID(uuid.New())}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "交換の2人以外", input: usecase.CompleteTradeInput{UserID: testutil.NewUserBuilder(t, db).Build(), TradeID: matched}, wantCode: model.AppErrCodeResourceNotFound},
		{name: "返事待ちの交換", input: usecase.CompleteTradeInput{UserID: receiverID, TradeID: pending}, wantCode: model.AppErrCodeConflict},
		{name: "やめた交換", input: usecase.CompleteTradeInput{UserID: receiverID, TradeID: cancelled, Note: "ありがとうございました"}, wantCode: model.AppErrCodeConflict},
		{name: "すでに押した交換", input: usecase.CompleteTradeInput{UserID: receiverID, TradeID: pressed, Note: "ありがとうございました"}, wantCode: model.AppErrCodeConflict},
		{name: "同意の無い人のひとこと", input: usecase.CompleteTradeInput{UserID: noConsentID, TradeID: withNoConsent, Note: "ありがとうございました"}, wantCode: model.AppErrCodeMessageConsentRequired},
	}
	for _, tt := range tests {
		_, err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != tt.wantCode {
			t.Errorf("%s: エラー = %v、コード %d を期待", tt.name, err, tt.wantCode)
		}
	}

	_, err := uc.Execute(t.Context(), usecase.CompleteTradeInput{UserID: receiverID, TradeID: matched, Note: strings.Repeat("あ", 1001)})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("note") {
		t.Errorf("1000文字を超えるひとこと: エラー = %v、ひとことの欄のエラーを期待", err)
	}

	for _, tradeID := range []model.TradeID{matched, pending, cancelled, pressed, withNoConsent} {
		if count := countTradeRows(t, "trade_events", tradeID) + countTradeRows(t, "trade_messages", tradeID); count != 0 {
			t.Errorf("交換 %s の出来事とメッセージの数 = %d、期待値 = 0", tradeID, count)
		}
	}
	for _, tradeID := range []model.TradeID{matched, withNoConsent} {
		trade, err := repository.NewTradeRepository(db).FindByID(t.Context(), tradeID)
		if err != nil || trade.Status != model.TradeStatusMatched || trade.ProposerCompletedAt != nil || trade.ReceiverCompletedAt != nil {
			t.Errorf("押せなかった交換 = (%+v, %v)、だれも押していないマッチ成立のままを期待", trade, err)
		}
	}
}

// TestCompleteTradeUsecase_Execute_Simultaneous は、2人がほぼ同時に押したとき、あとの更新が先に押された時刻を見て、
// 交換を1回だけ終えることを検証する。
func TestCompleteTradeUsecase_Execute_Simultaneous(t *testing.T) {
	t.Parallel()

	f := newCompletionFixture(t)
	db := testutil.GetTestDB()
	ctx, cancel := context.WithTimeout(jaContext(), 5*time.Second)
	defer cancel()

	// 申し込んだ人が押した更新をコミットせずに保ち、申し込まれた人の押す操作を待たせる。
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	first, err := repository.NewTradeRepository(db).WithTx(tx).Complete(ctx, f.tradeID, f.proposerID)
	if err != nil || first == nil || first.Status != model.TradeStatusMatched {
		t.Fatalf("先の Complete() = (%+v, %v)、マッチ成立のままの交換を期待", first, err)
	}

	type result struct {
		output *usecase.CompleteTradeOutput
		err    error
	}
	second := make(chan result, 1)
	go func() {
		output, err := newCompleteTradeUsecase().Execute(ctx, usecase.CompleteTradeInput{UserID: f.receiverID, TradeID: f.tradeID})
		second <- result{output: output, err: err}
	}()
	select {
	case r := <-second:
		t.Fatalf("先の更新のコミット前に押す操作が完了した: (%+v, %v)", r.output, r.err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	r := <-second
	if r.err != nil || !r.output.Completed {
		t.Fatalf("あとの Execute() = (%+v, %v)、交換を終える結果を期待", r.output, r.err)
	}
	trade, err := repository.NewTradeRepository(db).FindByID(ctx, f.tradeID)
	if err != nil || trade.Status != model.TradeStatusCompleted {
		t.Errorf("2人が押したあとの交換 = (%+v, %v)、「交換できた」を期待", trade, err)
	}
	assertItem(t, "申し込んだ人の譲れるアイテム", f.proposerGive, 1, model.ItemStatusListed)
}
