package repository_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestItemRepository_Create は、アイテムをリストにある状態で作ることと、同じリストに同じグッズを2つ入れようとすると
// ErrItemAlreadyListed を返すが、別のリストや、外したアイテムと同じグッズなら入れられることを検証する。
func TestItemRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	ctx := context.Background()
	userID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()

	created, err := repo.Create(ctx, userID, goodsID, repository.ItemAttributes{Kind: model.ItemKindGive, Quantity: 2, Note: "未開封"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if created.UserID != userID || created.GoodsID != goodsID || created.Kind != model.ItemKindGive || created.Status != model.ItemStatusListed || created.Quantity != 2 || created.Note != "未開封" {
		t.Errorf("作ったアイテム = %+v、渡した属性でリストにあるアイテムを期待", created)
	}

	if _, err := repo.Create(ctx, userID, goodsID, repository.ItemAttributes{Kind: model.ItemKindWant, Quantity: 1}); err != nil {
		t.Errorf("別のリストへのCreate()のエラー = %v、入れられることを期待", err)
	}

	removedGoodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()
	testutil.NewItemBuilder(t, tx, userID, removedGoodsID).WithRemoved().Build()
	if _, err := repo.Create(ctx, userID, removedGoodsID, repository.ItemAttributes{Kind: model.ItemKindGive, Quantity: 1}); err != nil {
		t.Errorf("外したアイテムと同じグッズのCreate()のエラー = %v、新しい行を作れることを期待", err)
	}

	// 一意制約の違反はテストのトランザクションを中断させるため、最後に確かめる。
	if _, err := repo.Create(ctx, userID, goodsID, repository.ItemAttributes{Kind: model.ItemKindGive, Quantity: 1}); !errors.Is(err, repository.ErrItemAlreadyListed) {
		t.Errorf("同じリストへの2つめのCreate()のエラー = %v、ErrItemAlreadyListed を期待", err)
	}
}

// TestItemRepository_ListListedByUserIDAndEventCategoryID は、指定したカテゴリーのグッズを指す、
// そのユーザーのリストにあるアイテムだけを返すことを検証する。
func TestItemRepository_ListListedByUserIDAndEventCategoryID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()

	giveID := testutil.NewItemBuilder(t, tx, userID, goodsID).Build()
	wantID := testutil.NewItemBuilder(t, tx, userID, goodsID).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithRemoved().Build()
	testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, eventID).Build()).Build()).Build()

	items, err := repo.ListListedByUserIDAndEventCategoryID(context.Background(), userID, categoryID)
	if err != nil {
		t.Fatalf("ListListedByUserIDAndEventCategoryID()のエラー = %v", err)
	}
	if len(items) != 2 || items[0].ID != giveID || items[1].ID != wantID {
		t.Errorf("アイテム = %+v、譲れる %s とほしい %s の2つを期待", items, giveID, wantID)
	}
}

// TestItemRepository_SumListedQuantities は、リストにあるアイテムの数量を、イベントごと・カテゴリーごとにリストを分けて合計し、
// 外したアイテムとほかのユーザーのアイテムを数えないことを検証する。
func TestItemRepository_SumListedQuantities(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	ctx := context.Background()
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	firstCategoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	secondCategoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()

	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, firstCategoryID).Build()).WithQuantity(2).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, secondCategoryID).Build()).WithQuantity(3).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, firstCategoryID).Build()).WithKind(model.ItemKindWant).WithQuantity(1).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, firstCategoryID).Build()).WithQuantity(5).WithRemoved().Build()
	testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), testutil.NewGoodsBuilder(t, tx, firstCategoryID).Build()).WithQuantity(7).Build()

	byEvent, err := repo.SumListedQuantitiesByUserIDGroupByEventID(ctx, userID)
	if err != nil {
		t.Fatalf("SumListedQuantitiesByUserIDGroupByEventID()のエラー = %v", err)
	}
	if got := byEvent[eventID]; got != (model.ItemQuantities{Give: 5, Want: 1}) || len(byEvent) != 1 {
		t.Errorf("イベントごとの合計 = %+v、イベント %s に譲れる5・ほしい1を期待", byEvent, eventID)
	}

	byCategory, err := repo.SumListedQuantitiesByUserIDAndEventIDGroupByEventCategoryID(ctx, userID, eventID)
	if err != nil {
		t.Fatalf("SumListedQuantitiesByUserIDAndEventIDGroupByEventCategoryID()のエラー = %v", err)
	}
	if byCategory[firstCategoryID] != (model.ItemQuantities{Give: 2, Want: 1}) || byCategory[secondCategoryID] != (model.ItemQuantities{Give: 3}) {
		t.Errorf("カテゴリーごとの合計 = %+v、1つめのカテゴリーに譲れる2・ほしい1、2つめのカテゴリーに譲れる3を期待", byCategory)
	}
}

// TestItemRepository_Exists は、グッズ・カテゴリー・イベントを参照するアイテムの有無を、外したアイテムと
// 公開していない配下のグッズからの参照も含めて返すことを検証する。
func TestItemRepository_Exists(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	ctx := context.Background()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).WithArchived("景品から外れたため").Build()
	testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).WithRemoved().Build()

	otherEventID := testutil.NewEventBuilder(t, tx).Build()
	otherCategoryID := testutil.NewEventCategoryBuilder(t, tx, otherEventID).Build()
	otherGoodsID := testutil.NewGoodsBuilder(t, tx, otherCategoryID).Build()

	checks := []struct {
		name string
		call func() (bool, error)
		want bool
	}{
		{name: "参照されているグッズ", call: func() (bool, error) { return repo.ExistsByGoodsID(ctx, goodsID) }, want: true},
		{name: "参照されているカテゴリー", call: func() (bool, error) { return repo.ExistsByEventCategoryID(ctx, categoryID) }, want: true},
		{name: "参照されているイベント", call: func() (bool, error) { return repo.ExistsByEventID(ctx, eventID) }, want: true},
		{name: "参照されていないグッズ", call: func() (bool, error) { return repo.ExistsByGoodsID(ctx, otherGoodsID) }, want: false},
		{name: "参照されていないカテゴリー", call: func() (bool, error) { return repo.ExistsByEventCategoryID(ctx, otherCategoryID) }, want: false},
		{name: "参照されていないイベント", call: func() (bool, error) { return repo.ExistsByEventID(ctx, otherEventID) }, want: false},
	}
	for _, check := range checks {
		got, err := check.call()
		if err != nil || got != check.want {
			t.Errorf("%s: (%v, %v)、(%v, nil) を期待", check.name, got, err, check.want)
		}
	}
}

// TestItemRepository_FindByID は、アイテムを状態を問わずに返すことと、無いIDには (nil, nil) を返すことを検証する。
func TestItemRepository_FindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	ctx := context.Background()
	userID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()
	removedID := testutil.NewItemBuilder(t, tx, userID, goodsID).WithKind(model.ItemKindWant).WithQuantity(3).WithNote("色違いでも可").WithRemoved().Build()

	found, err := repo.FindByID(ctx, removedID)
	if err != nil || found == nil {
		t.Fatalf("FindByID() = (%v, %v)、アイテムを期待", found, err)
	}
	if found.UserID != userID || found.GoodsID != goodsID || found.Kind != model.ItemKindWant || found.Status != model.ItemStatusRemoved || found.Quantity != 3 || found.Note != "色違いでも可" {
		t.Errorf("FindByID() = %+v、ほしいリストから外した3点・「色違いでも可」を期待", found)
	}

	missing, err := repo.FindByID(ctx, model.ItemID(uuid.New()))
	if missing != nil || err != nil {
		t.Errorf("無いIDの FindByID() = (%v, %v)、(nil, nil) を期待", missing, err)
	}
}

// TestItemRepository_ListListedByUserIDAndKind は、ユーザーのリストにあるアイテムのうち指定したリストのものだけを、
// イベントの開始日の新しい順・カテゴリーとグッズの並び順に、アーカイブしたマスタのものも含めて返すことを検証する。
func TestItemRepository_ListListedByUserIDAndKind(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	olderEventID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 1, 1), nil).Build()
	newerEventID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 2, 1), nil).WithArchived("終わったため").Build()
	secondCategoryID := testutil.NewEventCategoryBuilder(t, tx, newerEventID).WithPosition(2).Build()
	firstCategoryID := testutil.NewEventCategoryBuilder(t, tx, newerEventID).WithPosition(1).Build()
	olderCategoryID := testutil.NewEventCategoryBuilder(t, tx, olderEventID).Build()

	olderID := testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, olderCategoryID).Build()).Build()
	thirdID := testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, secondCategoryID).Build()).Build()
	secondID := testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, firstCategoryID).WithPosition(2).Build()).Build()
	firstID := testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, firstCategoryID).WithPosition(1).Build()).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, olderCategoryID).Build()).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, olderCategoryID).Build()).WithRemoved().Build()
	testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), testutil.NewGoodsBuilder(t, tx, olderCategoryID).Build()).Build()

	items, err := repo.ListListedByUserIDAndKind(context.Background(), userID, model.ItemKindGive)
	if err != nil {
		t.Fatalf("ListListedByUserIDAndKind()のエラー = %v", err)
	}
	want := []model.ItemID{firstID, secondID, thirdID, olderID}
	if len(items) != len(want) {
		t.Fatalf("アイテムの数 = %d、%d を期待", len(items), len(want))
	}
	for i, item := range items {
		if item.ID != want[i] {
			t.Errorf("%d番目のアイテム = %s、%s を期待", i, item.ID, want[i])
		}
	}
}

// TestItemRepository_SumListedQuantitiesByUserID は、リストにあるアイテムの数量をリストごとに合計し、
// 外したアイテムとほかのユーザーのアイテムを数えないことを検証する。
func TestItemRepository_SumListedQuantitiesByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithQuantity(2).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithQuantity(3).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithQuantity(5).WithRemoved().Build()
	testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithQuantity(7).Build()

	sum, err := repo.SumListedQuantitiesByUserID(context.Background(), userID)
	if err != nil || sum != (model.ItemQuantities{Give: 5, Want: 1}) {
		t.Errorf("SumListedQuantitiesByUserID() = (%+v, %v)、譲れる5・ほしい1を期待", sum, err)
	}

	empty, err := repo.SumListedQuantitiesByUserID(context.Background(), testutil.NewUserBuilder(t, tx).Build())
	if err != nil || empty != (model.ItemQuantities{}) {
		t.Errorf("アイテムが無いユーザーの SumListedQuantitiesByUserID() = (%+v, %v)、0を期待", empty, err)
	}
}

// TestItemRepository_UpdateListedAndRemoveListed は、ユーザーのリストにあるアイテムだけの数量とひとことを更新してリストから外せ、
// ほかのユーザーのアイテムと外したアイテムには何もしないことを検証する。
func TestItemRepository_UpdateListedAndRemoveListed(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	ctx := context.Background()
	userID := testutil.NewUserBuilder(t, tx).Build()
	otherUserID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()
	itemID := testutil.NewItemBuilder(t, tx, userID, goodsID).Build()

	if updated, err := repo.UpdateListed(ctx, itemID, otherUserID, 0, repository.ItemUpdateAttributes{Quantity: 5, Note: "未開封"}); updated || err != nil {
		t.Errorf("ほかのユーザーの UpdateListed() = (%v, %v)、(false, nil) を期待", updated, err)
	}
	if updated, err := repo.UpdateListed(ctx, itemID, userID, 0, repository.ItemUpdateAttributes{Quantity: 5, Note: "未開封"}); !updated || err != nil {
		t.Fatalf("UpdateListed() = (%v, %v)、(true, nil) を期待", updated, err)
	}
	if item, _ := repo.FindByID(ctx, itemID); item.Quantity != 5 || item.Note != "未開封" || item.LockVersion != 1 {
		t.Errorf("更新後のアイテム = %+v、5点・「未開封」・版1を期待", item)
	}
	if updated, err := repo.UpdateListed(ctx, itemID, userID, 0, repository.ItemUpdateAttributes{Quantity: 1}); updated || err != nil {
		t.Errorf("古い版の UpdateListed() = (%v, %v)、(false, nil) を期待", updated, err)
	}

	if removed, err := repo.RemoveListed(ctx, itemID, otherUserID, 1); removed || err != nil {
		t.Errorf("ほかのユーザーの RemoveListed() = (%v, %v)、(false, nil) を期待", removed, err)
	}
	if removed, err := repo.RemoveListed(ctx, itemID, userID, 0); removed || err != nil {
		t.Errorf("古い版の RemoveListed() = (%v, %v)、(false, nil) を期待", removed, err)
	}
	if removed, err := repo.RemoveListed(ctx, itemID, userID, 1); !removed || err != nil {
		t.Fatalf("RemoveListed() = (%v, %v)、(true, nil) を期待", removed, err)
	}
	if item, _ := repo.FindByID(ctx, itemID); item.Status != model.ItemStatusRemoved || item.Quantity != 5 || item.LockVersion != 2 {
		t.Errorf("外したあとのアイテム = %+v、数量を残して外した版2を期待", item)
	}

	if removed, err := repo.RemoveListed(ctx, itemID, userID, 2); removed || err != nil {
		t.Errorf("外したアイテムの RemoveListed() = (%v, %v)、(false, nil) を期待", removed, err)
	}
	if updated, err := repo.UpdateListed(ctx, itemID, userID, 2, repository.ItemUpdateAttributes{Quantity: 1}); updated || err != nil {
		t.Errorf("外したアイテムの UpdateListed() = (%v, %v)、(false, nil) を期待", updated, err)
	}
}

// TestItemRepository_RemoveListedByUserID は、ユーザーのリストにあるアイテムを譲れる・ほしいを問わずすべて外し、
// 行と数量は残すこと、ほかのユーザーのアイテムには触れないことを検証する。
func TestItemRepository_RemoveListedByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	ctx := context.Background()
	userID := testutil.NewUserBuilder(t, tx).Build()
	otherUserID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()
	giveID := testutil.NewItemBuilder(t, tx, userID, goodsID).WithQuantity(2).Build()
	wantID := testutil.NewItemBuilder(t, tx, userID, goodsID).WithKind(model.ItemKindWant).Build()
	otherID := testutil.NewItemBuilder(t, tx, otherUserID, goodsID).Build()

	if err := repo.RemoveListedByUserID(ctx, userID); err != nil {
		t.Fatalf("RemoveListedByUserID()のエラー = %v", err)
	}
	for _, id := range []model.ItemID{giveID, wantID} {
		if item, err := repo.FindByID(ctx, id); err != nil || item == nil || item.Status != model.ItemStatusRemoved || item.LockVersion != 1 {
			t.Errorf("ユーザーのアイテム = (%+v, %v)、外した版1を期待", item, err)
		}
	}
	if item, _ := repo.FindByID(ctx, giveID); item.Quantity != 2 {
		t.Errorf("外したアイテムの数量 = %d、2のままを期待", item.Quantity)
	}
	if item, err := repo.FindByID(ctx, otherID); err != nil || item == nil || item.Status != model.ItemStatusListed {
		t.Errorf("ほかのユーザーのアイテム = (%+v, %v)、リストにあるままを期待", item, err)
	}
}

// TestMasterRepositories_ListByIDs は、グッズ・カテゴリー・イベントを、指定したIDのものだけ状態を問わずにまとめて返し、
// 空のIDにはnilを返すことを検証する。
func TestMasterRepositories_ListByIDs(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	eventID := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()
	testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithArchived("景品から外れたため").Build()
	testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).WithArchived("景品から外れたため").Build()
	testutil.NewGoodsBuilder(t, tx, categoryID).Build()

	events, err := repository.NewEventRepository(db).WithTx(tx).ListByIDs(ctx, []model.EventID{eventID})
	if err != nil || len(events) != 1 || events[0].ID != eventID {
		t.Errorf("EventRepository.ListByIDs() = (%v, %v)、イベント %s だけを期待", events, err, eventID)
	}
	categories, err := repository.NewEventCategoryRepository(db).WithTx(tx).ListByIDs(ctx, []model.EventCategoryID{categoryID})
	if err != nil || len(categories) != 1 || categories[0].ID != categoryID {
		t.Errorf("EventCategoryRepository.ListByIDs() = (%v, %v)、カテゴリー %s だけを期待", categories, err, categoryID)
	}
	goods, err := repository.NewGoodsRepository(db).WithTx(tx).ListByIDs(ctx, []model.GoodsID{goodsID})
	if err != nil || len(goods) != 1 || goods[0].ID != goodsID {
		t.Errorf("GoodsRepository.ListByIDs() = (%v, %v)、グッズ %s だけを期待", goods, err, goodsID)
	}

	if empty, err := repository.NewGoodsRepository(db).WithTx(tx).ListByIDs(ctx, nil); empty != nil || err != nil {
		t.Errorf("空のIDの ListByIDs() = (%v, %v)、(nil, nil) を期待", empty, err)
	}
}

// TestItemRepository_ListListedMatching は、ユーザーのリストにあるアイテムのうち、相手のだれかが同じグッズを反対のリストに入れているものを、
// リストと同じ並びで返すことと、どちらかのユーザーが空ならクエリを発行しないことを検証する。
// 同じリストに入れているだけのグッズと、リストから外したアイテムは含めない。
func TestItemRepository_ListListedMatching(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewItemRepository(db).WithTx(tx)
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()
	first := testutil.NewGoodsBuilder(t, tx, categoryID).WithPosition(1).Build()
	second := testutil.NewGoodsBuilder(t, tx, categoryID).WithPosition(2).Build()
	third := testutil.NewGoodsBuilder(t, tx, categoryID).WithPosition(3).Build()
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()

	secondGive := testutil.NewItemBuilder(t, tx, userID, second).Build()
	firstWant := testutil.NewItemBuilder(t, tx, userID, first).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, tx, userID, third).Build()
	testutil.NewItemBuilder(t, tx, partnerID, second).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, tx, partnerID, first).Build()
	testutil.NewItemBuilder(t, tx, partnerID, third).Build()
	testutil.NewItemBuilder(t, tx, partnerID, third).WithKind(model.ItemKindWant).WithRemoved().Build()

	items, err := repo.ListListedMatching(ctx, []model.UserID{userID}, []model.UserID{partnerID})
	if err != nil {
		t.Fatalf("ListListedMatching()のエラー = %v", err)
	}
	if len(items) != 2 || items[0].ID != firstWant || items[1].ID != secondGive {
		t.Errorf("ListListedMatching() = %v、ほしい1番目と譲れる2番目のグッズのアイテムをグッズの並び順に期待", items)
	}

	for _, ids := range [][2][]model.UserID{{nil, {partnerID}}, {{userID}, nil}} {
		if items, err := repo.ListListedMatching(ctx, ids[0], ids[1]); err != nil || items != nil {
			t.Errorf("空のユーザーでの ListListedMatching() = (%v, %v)、(nil, nil) を期待", items, err)
		}
	}
}

// TestItemRepository_LockByIDs は、指定したIDのアイテムを状態と持ち主を問わずにID順で返し、空のIDではクエリを発行せずにnilを返すことを検証する。
func TestItemRepository_LockByIDs(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewItemRepository(db).WithTx(tx)
	ctx := context.Background()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()
	listedID := testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).Build()
	removedID := testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).WithRemoved().Build()
	testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).Build()

	items, err := repo.LockByIDs(ctx, []model.ItemID{removedID, listedID})
	if err != nil {
		t.Fatalf("LockByIDs()のエラー = %v", err)
	}
	want := []model.ItemID{listedID, removedID}
	slices.SortFunc(want, func(a, b model.ItemID) int { return bytes.Compare(a[:], b[:]) })
	if len(items) != 2 || items[0].ID != want[0] || items[1].ID != want[1] {
		t.Errorf("LockByIDs() = %v、指定した2つのアイテムをID順に期待 (%v)", items, want)
	}

	if items, err := repo.LockByIDs(ctx, nil); err != nil || items != nil {
		t.Errorf("LockByIDs(nil) = (%v, %v)、(nil, nil) を期待", items, err)
	}
}

// TestItemRepository_ListByIDs は、指定したIDのアイテムを、リストから外したものも含めてまとめて返すことを検証する。
func TestItemRepository_ListByIDs(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewItemRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()
	listedID := testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).Build()
	removedID := testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithRemoved().Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).Build()

	items, err := repo.ListByIDs(ctx, []model.ItemID{listedID, removedID})
	if err != nil {
		t.Fatalf("ListByIDs()のエラー = %v", err)
	}
	gotIDs := make([]model.ItemID, len(items))
	for i, item := range items {
		gotIDs[i] = item.ID
	}
	if len(gotIDs) != 2 || !slices.Contains(gotIDs, listedID) || !slices.Contains(gotIDs, removedID) {
		t.Errorf("ListByIDs()のアイテム = %v、リストにあるものと外したものの2つを期待", gotIDs)
	}

	if items, err := repo.ListByIDs(ctx, nil); err != nil || items != nil {
		t.Errorf("空: ListByIDs() = (%v, %v)、(nil, nil) を期待", items, err)
	}
}
