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

// newCreateTradeUsecase は、自分でトランザクションを開く CreateTradeUsecase をテスト用のデータベースで組み立てる。
func newCreateTradeUsecase() *usecase.CreateTradeUsecase {
	db := testutil.GetTestDB()

	return usecase.NewCreateTradeUsecase(
		db,
		validator.NewTradeCreateValidator(repository.NewItemRepository(db)),
		repository.NewMessageConsentRepository(db),
		repository.NewTradeRepository(db),
		repository.NewTradeEventRepository(db),
		repository.NewTradeItemRepository(db),
		repository.NewTradeMessageRepository(db),
		repository.NewUserRepository(db),
	)
}

// tradeFixture は交換の申し込みのテストに使う、コミットした2人のユーザーと、それぞれの譲れるアイテム。
type tradeFixture struct {
	proposer *model.User
	receiver *model.User
	// receiveItemID は申し込まれた人の譲れるアイテム、giveItemID は申し込んだ人の譲れるアイテム。
	receiveItemID model.ItemID
	giveItemID    model.ItemID
}

// newTradeFixture は、同意済みの申し込む人と、申し込まれる人と、2人の譲れるアイテムをコミットして作る。
func newTradeFixture(t *testing.T) tradeFixture {
	t.Helper()

	db := testutil.GetTestDB()
	categoryID := testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()
	proposer := newCommittedUserWithRole(t, model.UserRoleUser)
	receiver := newCommittedUserWithRole(t, model.UserRoleUser)
	testutil.NewMessageConsentBuilder(t, db, proposer.ID).Build()

	return tradeFixture{
		proposer:      proposer,
		receiver:      receiver,
		receiveItemID: testutil.NewItemBuilder(t, db, receiver.ID, testutil.NewGoodsBuilder(t, db, categoryID).Build()).Build(),
		giveItemID:    testutil.NewItemBuilder(t, db, proposer.ID, testutil.NewGoodsBuilder(t, db, categoryID).Build()).Build(),
	}
}

// input はこのフィクスチャーの2人とアイテムで申し込む入力を返す。
func (f tradeFixture) input(note string) usecase.CreateTradeInput {
	return usecase.CreateTradeInput{
		ProposerUserID: f.proposer.ID,
		Atname:         f.receiver.Atname,
		ReceiveItemIDs: []string{f.receiveItemID.String()},
		GiveItemIDs:    []string{f.giveItemID.String()},
		Note:           note,
	}
}

// countTradeRows は、交換 tradeID を指す table の行の数を返す。
func countTradeRows(t *testing.T, table string, tradeID model.TradeID) int {
	t.Helper()

	var count int
	if err := testutil.GetTestDB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table+" WHERE trade_id = $1", uuid.UUID(tradeID)).Scan(&count); err != nil {
		t.Fatalf("%sの行の数の取得に失敗しました: %v", table, err)
	}

	return count
}

// TestCreateTradeUsecase_Execute は、返事待ちの交換を作り、選んだアイテムを交換の品に、申し込みを出来事に記録し、
// 前後の空白を除いたひとことをメッセージの1通目として残すことを検証する。アットネームは大文字小文字を区別しない。
func TestCreateTradeUsecase_Execute(t *testing.T) {
	t.Parallel()

	f := newTradeFixture(t)
	input := f.input("  はじめまして。土日の昼なら都内で動けます。\n")
	input.Atname = strings.ToUpper(f.receiver.Atname)
	input.GiveItemIDs = append(input.GiveItemIDs, f.giveItemID.String())

	output, err := newCreateTradeUsecase().Execute(jaContext(), input)
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	trade := output.Trade
	if trade.ProposerUserID != f.proposer.ID || trade.ReceiverUserID != f.receiver.ID || trade.Status != model.TradeStatusPending || output.Receiver.ID != f.receiver.ID {
		t.Errorf("Execute() = %+v、申し込んだ人から申し込まれた人への返事待ちの交換を期待", output)
	}

	db := testutil.GetTestDB()
	rows, err := db.QueryContext(context.Background(), "SELECT item_id FROM trade_items WHERE trade_id = $1", uuid.UUID(trade.ID))
	if err != nil {
		t.Fatalf("交換の品の取得に失敗しました: %v", err)
	}
	defer func() { _ = rows.Close() }()
	items := map[model.ItemID]int{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("交換の品の読み取りに失敗しました: %v", err)
		}
		items[model.ItemID(id)]++
	}
	// 同じアイテムを2度送られても1点にまとめる。
	if len(items) != 2 || items[f.receiveItemID] != 1 || items[f.giveItemID] != 1 {
		t.Errorf("交換の品 = %v、もらうものと渡すものの1点ずつを期待", items)
	}

	var kind string
	var actorID uuid.UUID
	if err := db.QueryRowContext(context.Background(), "SELECT kind, actor_user_id FROM trade_events WHERE trade_id = $1", uuid.UUID(trade.ID)).Scan(&kind, &actorID); err != nil || kind != "proposed" || model.UserID(actorID) != f.proposer.ID {
		t.Errorf("出来事 = (%s, %s, %v)、申し込んだ人の申し込みを期待", kind, actorID, err)
	}

	var body string
	var senderID uuid.UUID
	if err := db.QueryRowContext(context.Background(), "SELECT body, sender_user_id FROM trade_messages WHERE trade_id = $1", uuid.UUID(trade.ID)).Scan(&body, &senderID); err != nil || body != "はじめまして。土日の昼なら都内で動けます。" || model.UserID(senderID) != f.proposer.ID {
		t.Errorf("メッセージ = (%q, %s, %v)、申し込んだ人の前後の空白を除いたひとことを期待", body, senderID, err)
	}
}

// TestCreateTradeUsecase_Execute_WithoutNote は、ひとことが空のときは、メッセージを残さずに申し込むことを検証する。
func TestCreateTradeUsecase_Execute_WithoutNote(t *testing.T) {
	t.Parallel()

	f := newTradeFixture(t)

	output, err := newCreateTradeUsecase().Execute(jaContext(), f.input(" \n "))
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if count := countTradeRows(t, "trade_messages", output.Trade.ID); count != 0 {
		t.Errorf("メッセージの数 = %d、0を期待", count)
	}
	if count := countTradeRows(t, "trade_events", output.Trade.ID); count != 1 {
		t.Errorf("出来事の数 = %d、1を期待", count)
	}
}

// TestCreateTradeUsecase_Execute_NotFound は、相手がいない・退会した・自分自身のときに AppErrCodeResourceNotFound を返すことを検証する。
func TestCreateTradeUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	f := newTradeFixture(t)
	withdrawnAtname := testutil.UniqueAtname()
	testutil.NewUserBuilder(t, testutil.GetTestDB()).WithAtname(withdrawnAtname).WithDeletedAt(time.Now()).Build()
	uc := newCreateTradeUsecase()

	for name, atname := range map[string]string{
		"いない":  testutil.UniqueAtname(),
		"退会した": withdrawnAtname,
		"自分自身": f.proposer.Atname,
	} {
		input := f.input("")
		input.Atname = atname
		_, err := uc.Execute(jaContext(), input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFoundを期待", name, err)
		}
	}
}

// TestCreateTradeUsecase_Execute_MessageConsentRequired は、申し込む人に有効な同意が無い (記録が無い・やめた・古い版) ときは、
// 申し込まずに AppErrCodeMessageConsentRequired を返すことを検証する。
func TestCreateTradeUsecase_Execute_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newCreateTradeUsecase()

	for name, consent := range map[string]func(userID model.UserID){
		"記録が無い": func(model.UserID) {},
		"やめた": func(userID model.UserID) {
			testutil.NewMessageConsentBuilder(t, db, userID).WithWithdrawnAt(time.Now()).Build()
		},
		"古い版": func(userID model.UserID) {
			testutil.NewMessageConsentBuilder(t, db, userID).WithVersion(model.CurrentMessageConsentVersion - 1).Build()
		},
	} {
		f := newTradeFixture(t)
		// フィクスチャーの同意より新しい記録で、有効な同意を無くす。記録が無い場合は、同意の無いユーザーに差し替える。
		if name == "記録が無い" {
			f.proposer = newCommittedUserWithRole(t, model.UserRoleUser)
			f.giveItemID = testutil.NewItemBuilder(t, db, f.proposer.ID, testutil.NewGoodsBuilder(t, db, newCommittedEventCategoryID(t)).Build()).Build()
		}
		consent(f.proposer.ID)

		_, err := uc.Execute(jaContext(), f.input(""))
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeMessageConsentRequired {
			t.Errorf("%s: エラー = %v、AppErrCodeMessageConsentRequiredを期待", name, err)
		}
		if exists, _ := repository.NewTradeRepository(db).ExistsInProgressByUserID(t.Context(), f.proposer.ID); exists {
			t.Errorf("%s: 同意が無いのに交換を作った", name)
		}
	}
}

// TestCreateTradeUsecase_Execute_ValidationErrors は、選び方の誤りと長すぎるひとことで *model.ValidationError を返し、申し込まないことを検証する。
// 選べるのは、もらうものは相手の、渡すものは自分の、譲れるリストにあるアイテムに限る。
func TestCreateTradeUsecase_Execute_ValidationErrors(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	f := newTradeFixture(t)
	categoryID := newCommittedEventCategoryID(t)
	removedItemID := testutil.NewItemBuilder(t, db, f.receiver.ID, testutil.NewGoodsBuilder(t, db, categoryID).Build()).WithRemoved().Build()
	wantItemID := testutil.NewItemBuilder(t, db, f.receiver.ID, testutil.NewGoodsBuilder(t, db, categoryID).Build()).WithKind(model.ItemKindWant).Build()
	uc := newCreateTradeUsecase()

	for name, tc := range map[string]struct {
		modify    func(input *usecase.CreateTradeInput)
		wantField string
	}{
		"もらうものが無い":      {modify: func(input *usecase.CreateTradeInput) { input.ReceiveItemIDs = nil }},
		"渡すものが無い":       {modify: func(input *usecase.CreateTradeInput) { input.GiveItemIDs = nil }},
		"読めないID":        {modify: func(input *usecase.CreateTradeInput) { input.ReceiveItemIDs = []string{"not-a-uuid"} }},
		"リストから外したアイテム":  {modify: func(input *usecase.CreateTradeInput) { input.ReceiveItemIDs = []string{removedItemID.String()} }},
		"ほしいリストのアイテム":   {modify: func(input *usecase.CreateTradeInput) { input.ReceiveItemIDs = []string{wantItemID.String()} }},
		"自分のアイテムをもらうもの": {modify: func(input *usecase.CreateTradeInput) { input.ReceiveItemIDs = []string{f.giveItemID.String()} }},
		"相手のアイテムを渡すもの":  {modify: func(input *usecase.CreateTradeInput) { input.GiveItemIDs = []string{f.receiveItemID.String()} }},
		"長すぎるひとこと": {
			modify:    func(input *usecase.CreateTradeInput) { input.Note = strings.Repeat("あ", 1001) },
			wantField: "note",
		},
	} {
		input := f.input("")
		tc.modify(&input)

		_, err := uc.Execute(jaContext(), input)
		ve := model.AsValidationError(err)
		if ve == nil {
			t.Errorf("%s: エラー = %v、ValidationErrorを期待", name, err)
			continue
		}
		if tc.wantField != "" && !ve.HasFieldError(tc.wantField) {
			t.Errorf("%s: エラー = %+v、%sのエラーを期待", name, ve, tc.wantField)
		}
		if tc.wantField == "" && len(ve.Global) == 0 {
			t.Errorf("%s: エラー = %+v、フォーム全体のエラーを期待", name, ve)
		}
	}
	if exists, _ := repository.NewTradeRepository(db).ExistsInProgressByUserID(t.Context(), f.proposer.ID); exists {
		t.Error("受け付けなかった申し込みで交換を作った")
	}
}

// newCommittedEventCategoryID はコミットしたイベントとそのカテゴリーを作り、カテゴリーのIDを返す。
func newCommittedEventCategoryID(t *testing.T) model.EventCategoryID {
	t.Helper()

	db := testutil.GetTestDB()

	return testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()
}
