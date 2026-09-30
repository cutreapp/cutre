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

// newGetTradeMessagesUsecase はテストのトランザクションで読む GetTradeMessagesUsecase を組み立てる。
func newGetTradeMessagesUsecase(db *sql.DB, tx *sql.Tx) *usecase.GetTradeMessagesUsecase {
	return usecase.NewGetTradeMessagesUsecase(
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewMessageConsentRepository(db).WithTx(tx),
		repository.NewTradeRepository(db).WithTx(tx),
		repository.NewTradeEventRepository(db).WithTx(tx),
		repository.NewTradeItemRepository(db).WithTx(tx),
		repository.NewTradeMessageRepository(db).WithTx(tx),
		repository.NewTradeMessageReadRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
}

// TestGetTradeMessagesUsecase_Execute は、交換の2人のどちらが開いても、相手・交換の品・メッセージ・出来事と、
// 開いた人の同意が有効かと、前にどこまで読んだかを返すことを検証する。退会した相手も交換の記録として返す。
func TestGetTradeMessagesUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetTradeMessagesUsecase(db, tx)
	categoryID := newEventCategoryID(t, tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()
	testutil.NewMessageConsentBuilder(t, tx, proposerID).Build()
	receiveItemID := testutil.NewItemBuilder(t, tx, receiverID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).Build()
	giveItemID := testutil.NewItemBuilder(t, tx, proposerID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, receiverID).Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), tradeID, []model.ItemID{receiveItemID, giveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	if _, err := repository.NewTradeEventRepository(db).WithTx(tx).Create(t.Context(), tradeID, proposerID, model.TradeEventKindProposed, nil); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	messageID := testutil.NewTradeMessageBuilder(t, tx, tradeID, proposerID, "よろしくお願いします").Build()

	output, err := uc.Execute(t.Context(), usecase.GetTradeMessagesInput{ViewerUserID: proposerID, TradeID: tradeID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Trade.ID != tradeID || output.Partner.ID != receiverID {
		t.Errorf("Trade = %+v, Partner = %+v、交換と申し込まれた人を期待", output.Trade, output.Partner)
	}
	if len(output.Items) != 2 || output.Items[0].ID != receiveItemID || output.Items[1].ID != giveItemID {
		t.Errorf("Items = %+v、入れた順の2点を期待", output.Items)
	}
	if len(output.Messages) != 1 || output.Messages[0].ID != messageID {
		t.Errorf("Messages = %+v、ひとことの1通を期待", output.Messages)
	}
	if len(output.Events) != 1 || output.Events[0].Kind != model.TradeEventKindProposed {
		t.Errorf("Events = %+v、申し込みの1件を期待", output.Events)
	}
	if !output.MessageConsentValid {
		t.Error("同意した申し込んだ人の MessageConsentValid = false、trueを期待")
	}
	if output.LastReadPosition != nil {
		t.Errorf("LastReadPosition = %v、読んだことが無いためnilを期待", output.LastReadPosition)
	}

	lastReadAt := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	if err := repository.NewTradeMessageReadRepository(db).WithTx(tx).Save(t.Context(), tradeID, receiverID, model.TradeMessageReadPosition{CreatedAt: lastReadAt, MessageID: messageID}); err != nil {
		t.Fatalf("Save()のエラー = %v", err)
	}
	output, err = uc.Execute(t.Context(), usecase.GetTradeMessagesInput{ViewerUserID: receiverID, TradeID: tradeID})
	if err != nil || output.Partner.ID != proposerID || output.MessageConsentValid {
		t.Errorf("申し込まれた人が開いたとき: (%+v, %v)、相手が申し込んだ人で、同意の無いことを期待", output, err)
	}
	if output.LastReadPosition == nil || !output.LastReadPosition.CreatedAt.Equal(lastReadAt) || output.LastReadPosition.MessageID != messageID {
		t.Errorf("LastReadPosition = %v、時刻 %v・メッセージ %s を期待", output.LastReadPosition, lastReadAt, messageID)
	}
}

// TestGetTradeMessagesUsecase_Execute_NotFound は、交換が無いときと、交換の2人以外が開いたときに AppErrCodeResourceNotFound を返すことを検証する。
func TestGetTradeMessagesUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetTradeMessagesUsecase(db, tx)
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, proposerID, testutil.NewUserBuilder(t, tx).Build()).Build()

	tests := []struct {
		name  string
		input usecase.GetTradeMessagesInput
	}{
		{name: "交換の2人以外", input: usecase.GetTradeMessagesInput{ViewerUserID: testutil.NewUserBuilder(t, tx).Build(), TradeID: tradeID}},
		{name: "無い交換", input: usecase.GetTradeMessagesInput{ViewerUserID: proposerID, TradeID: model.TradeID(uuid.New())}},
	}
	for _, tt := range tests {
		_, err := uc.Execute(t.Context(), tt.input)
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: Execute()のエラー = %v、AppErrCodeResourceNotFound を期待", tt.name, err)
		}
	}
}
