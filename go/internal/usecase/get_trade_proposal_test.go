package usecase_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newGetTradeProposalUsecase はテストのトランザクションで読む GetTradeProposalUsecase を組み立てる。
func newGetTradeProposalUsecase(db *sql.DB, tx *sql.Tx) *usecase.GetTradeProposalUsecase {
	return usecase.NewGetTradeProposalUsecase(
		repository.NewEventCategoryRepository(db).WithTx(tx),
		repository.NewGoodsRepository(db).WithTx(tx),
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewMessageConsentRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
}

// TestGetTradeProposalUsecase_Execute は、申し込む相手と、その人との間で交換できるアイテムと、申し込む人の同意の有無を返すことを検証する。
// アットネームは大文字小文字を区別しない。
func TestGetTradeProposalUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetTradeProposalUsecase(db, tx)
	categoryID := newEventCategoryID(t, tx)
	receivableGoods := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	givableGoods := testutil.NewGoodsBuilder(t, tx, categoryID).Build()

	proposerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewItemBuilder(t, tx, proposerID, receivableGoods).WithKind(model.ItemKindWant).Build()
	givableItemID := testutil.NewItemBuilder(t, tx, proposerID, givableGoods).Build()

	atname := testutil.UniqueAtname()
	receiverID := testutil.NewUserBuilder(t, tx).WithAtname(atname).Build()
	receivableItemID := testutil.NewItemBuilder(t, tx, receiverID, receivableGoods).Build()
	testutil.NewItemBuilder(t, tx, receiverID, givableGoods).WithKind(model.ItemKindWant).Build()

	output, err := uc.Execute(t.Context(), usecase.GetTradeProposalInput{ProposerUserID: proposerID, Atname: strings.ToUpper(atname)})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Receiver.ID != receiverID {
		t.Errorf("Receiver = %+v、%s を期待", output.Receiver, receiverID)
	}
	if len(output.Match.Receivable) != 1 || output.Match.Receivable[0].Item.ID != receivableItemID || len(output.Match.Givable) != 1 || output.Match.Givable[0].Item.ID != givableItemID {
		t.Errorf("Match = %+v、もらえるものと渡せるものの1つずつを期待", output.Match)
	}
	if output.Goods[receivableGoods] == nil || output.EventCategories[categoryID] == nil {
		t.Errorf("Goods = %v, EventCategories = %v、交換できるアイテムのグッズとカテゴリーを期待", output.Goods, output.EventCategories)
	}
	if output.MessageConsentValid {
		t.Error("同意の記録が無いのに、MessageConsentValid がtrue")
	}

	testutil.NewMessageConsentBuilder(t, tx, proposerID).Build()
	output, err = uc.Execute(t.Context(), usecase.GetTradeProposalInput{ProposerUserID: proposerID, Atname: atname})
	if err != nil || !output.MessageConsentValid {
		t.Errorf("同意したあと: (%+v, %v)、MessageConsentValid がtrueを期待", output, err)
	}
}

// TestGetTradeProposalUsecase_Execute_NotFound は、相手がいない・退会した・自分自身のときに AppErrCodeResourceNotFound を返すことを検証する。
func TestGetTradeProposalUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetTradeProposalUsecase(db, tx)
	proposerAtname := testutil.UniqueAtname()
	proposerID := testutil.NewUserBuilder(t, tx).WithAtname(proposerAtname).Build()
	withdrawnAtname := testutil.UniqueAtname()
	testutil.NewUserBuilder(t, tx).WithAtname(withdrawnAtname).WithDeletedAt(time.Now()).Build()

	for name, atname := range map[string]string{
		"いない":  testutil.UniqueAtname(),
		"退会した": withdrawnAtname,
		"自分自身": proposerAtname,
	} {
		_, err := uc.Execute(t.Context(), usecase.GetTradeProposalInput{ProposerUserID: proposerID, Atname: atname})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFoundを期待", name, err)
		}
	}
}
