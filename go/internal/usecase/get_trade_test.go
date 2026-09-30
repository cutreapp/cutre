package usecase_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newGetTradeUsecase はテストのトランザクションで読む GetTradeUsecase を組み立てる。
func newGetTradeUsecase(db *sql.DB, tx *sql.Tx) *usecase.GetTradeUsecase {
	return usecase.NewGetTradeUsecase(
		repository.NewEventCategoryRepository(db).WithTx(tx),
		repository.NewGoodsRepository(db).WithTx(tx),
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewMessageConsentRepository(db).WithTx(tx),
		repository.NewTradeRepository(db).WithTx(tx),
		repository.NewTradeEventRepository(db).WithTx(tx),
		repository.NewTradeItemRepository(db).WithTx(tx),
		repository.NewTradeMessageRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
}

// TestGetTradeUsecase_Execute は、交換の2人のどちらが開いても、相手・交換の品・これまでの流れ・最新のメッセージ・未読の数と、
// 開いたユーザーのメッセージの取り扱いへの同意の有無を返すことを検証する。
// リストから外したアイテムと、退会した相手も交換の記録として返す。
func TestGetTradeUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetTradeUsecase(db, tx)
	categoryID := newEventCategoryID(t, tx)
	receiveGoods := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	giveGoods := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()
	testutil.NewMessageConsentBuilder(t, tx, proposerID).Build()
	receiveItemID := testutil.NewItemBuilder(t, tx, receiverID, receiveGoods).Build()
	giveItemID := testutil.NewItemBuilder(t, tx, proposerID, giveGoods).WithRemoved().Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), tradeID, []model.ItemID{receiveItemID, giveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	if _, err := repository.NewTradeEventRepository(db).WithTx(tx).Create(t.Context(), tradeID, proposerID, model.TradeEventKindProposed, nil); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	testutil.NewTradeMessageBuilder(t, tx, tradeID, proposerID, "よろしくお願いします").WithCreatedAt(time.Now().Add(-time.Minute)).Build()
	latestID := testutil.NewTradeMessageBuilder(t, tx, tradeID, receiverID, "こちらこそ").Build()

	output, err := uc.Execute(t.Context(), usecase.GetTradeInput{ViewerUserID: proposerID, TradeID: tradeID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Trade.ID != tradeID || output.Partner.ID != receiverID {
		t.Errorf("Trade = %+v, Partner = %+v、交換と申し込まれた人を期待", output.Trade, output.Partner)
	}
	if len(output.Items) != 2 || output.Items[0].ID != receiveItemID || output.Items[1].ID != giveItemID {
		t.Errorf("Items = %+v、入れた順の2点を期待", output.Items)
	}
	if output.Goods[receiveGoods] == nil || output.Goods[giveGoods] == nil || output.EventCategories[categoryID] == nil {
		t.Errorf("Goods = %v, EventCategories = %v、交換の品のグッズとカテゴリーを期待", output.Goods, output.EventCategories)
	}
	if len(output.Events) != 1 || output.Events[0].Kind != model.TradeEventKindProposed {
		t.Errorf("Events = %+v、申し込みの1件を期待", output.Events)
	}
	if output.LatestMessage == nil || output.LatestMessage.ID != latestID || output.UnreadMessageCount != 1 {
		t.Errorf("LatestMessage = %+v, UnreadMessageCount = %d、相手の最新の1通と未読の1通を期待", output.LatestMessage, output.UnreadMessageCount)
	}
	if !output.MessageConsentValid {
		t.Error("MessageConsentValid = false、同意した申し込んだ人ではtrueを期待")
	}

	output, err = uc.Execute(t.Context(), usecase.GetTradeInput{ViewerUserID: receiverID, TradeID: tradeID})
	if err != nil || output.Partner.ID != proposerID || output.UnreadMessageCount != 1 || output.MessageConsentValid {
		t.Errorf("申し込まれた人が開いたとき: (%+v, %v)、相手が申し込んだ人で、未読の1通と、同意の無いことを期待", output, err)
	}
}

// TestGetTradeUsecase_Execute_NotFound は、交換が無いときと、交換の2人以外が開いたときに AppErrCodeResourceNotFound を返すことを検証する。
func TestGetTradeUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetTradeUsecase(db, tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, testutil.NewUserBuilder(t, tx).Build()).Build()

	tests := []struct {
		name  string
		input usecase.GetTradeInput
	}{
		{name: "交換の2人以外", input: usecase.GetTradeInput{ViewerUserID: testutil.NewUserBuilder(t, tx).Build(), TradeID: tradeID}},
		{name: "無い交換", input: usecase.GetTradeInput{ViewerUserID: proposerID, TradeID: model.TradeID(uuid.New())}},
	}
	for _, tt := range tests {
		_, err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFound を期待", tt.name, err)
		}
	}
}
