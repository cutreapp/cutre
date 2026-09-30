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

// newGetProfileUsecase はテストのトランザクションで読む GetProfileUsecase を組み立てる。
func newGetProfileUsecase(db *sql.DB, tx *sql.Tx) *usecase.GetProfileUsecase {
	return usecase.NewGetProfileUsecase(
		repository.NewEventCategoryRepository(db).WithTx(tx),
		repository.NewGoodsRepository(db).WithTx(tx),
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewStationRepository(db).WithTx(tx),
		repository.NewTradeRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
}

// TestGetProfileUsecase_Execute は、アットネームのユーザーの交換場所・「交換できた」の件数と、見ているユーザーとの間で交換できるアイテムを返すことを検証する。
// 「交換できた」の件数は、見ているユーザー以外との交換も数え、ほかの段階で終わった交換は数えない。
// 交換場所の都道府県が違っても、交換できるアイテムは返す。アットネームは大文字小文字を区別しない。
func TestGetProfileUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetProfileUsecase(db, tx)
	categoryID := newEventCategoryID(t, tx)
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()

	viewerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserStationBuilder(t, tx, viewerID, testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).Build()).Build()
	testutil.NewItemBuilder(t, tx, viewerID, goodsID).WithKind(model.ItemKindWant).Build()

	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, tx).WithAtname(atname).Build()
	stationID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(27).Build()
	testutil.NewUserStationBuilder(t, tx, userID, stationID).Build()
	itemID := testutil.NewItemBuilder(t, tx, userID, goodsID).Build()
	testutil.NewTradeBuilder(t, tx, userID, viewerID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), userID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeBuilder(t, tx, userID, viewerID).WithStatus(model.TradeStatusFailed).Build()

	output, err := uc.Execute(t.Context(), usecase.GetProfileInput{ViewerUserID: viewerID, Atname: strings.ToUpper(atname)})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.User.ID != userID || len(output.Stations) != 1 || output.Stations[0].ID != stationID {
		t.Errorf("Execute() = %+v、ユーザーとその1駅を期待", output)
	}
	if output.CompletedTradeCount != 2 {
		t.Errorf("CompletedTradeCount = %d、期待値 = 2", output.CompletedTradeCount)
	}
	if len(output.Match.Receivable) != 1 || output.Match.Receivable[0].Item.ID != itemID || len(output.Match.Givable) != 0 {
		t.Errorf("Match = %+v、もらえるもの1つ・渡せるもの無しを期待", output.Match)
	}
	if output.Goods[goodsID] == nil || output.EventCategories[categoryID] == nil {
		t.Errorf("Goods = %v, EventCategories = %v、交換できるアイテムのグッズとカテゴリーを期待", output.Goods, output.EventCategories)
	}
}

// TestGetProfileUsecase_Execute_NotFound は、いないユーザー・退会したユーザー・見ているユーザー自身を、無いものとして扱うことを検証する。
func TestGetProfileUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetProfileUsecase(db, tx)
	viewerAtname := testutil.UniqueAtname()
	viewerID := testutil.NewUserBuilder(t, tx).WithAtname(viewerAtname).Build()
	withdrawnAtname := testutil.UniqueAtname()
	testutil.NewUserBuilder(t, tx).WithAtname(withdrawnAtname).WithDeletedAt(time.Now()).Build()

	for name, atname := range map[string]string{"いない": testutil.UniqueAtname(), "退会した": withdrawnAtname, "自分": viewerAtname} {
		_, err := uc.Execute(t.Context(), usecase.GetProfileInput{ViewerUserID: viewerID, Atname: atname})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: Execute()のエラー = %v、AppErrCodeResourceNotFound を期待", name, err)
		}
	}
}
