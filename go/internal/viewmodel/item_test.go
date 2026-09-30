package viewmodel_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestNewItemEditForm は、保存済みのアイテムのリスト・数量・ひとことをフォームの値にすることを検証する。
func TestNewItemEditForm(t *testing.T) {
	t.Parallel()

	form := viewmodel.NewItemEditForm(&model.Item{Kind: model.ItemKindWant, Quantity: 12, Note: "色違いでも可", LockVersion: 3})
	if form != (viewmodel.ItemForm{Kind: "want", Quantity: "12", Note: "色違いでも可", LockVersion: 3}) {
		t.Errorf("フォームの値 = %+v、ほしい・12・「色違いでも可」・版3を期待", form)
	}
}

// TestNewListSections は、並んだアイテムをイベントごとの欄にまとめ、欄ごとに数量を合計することを検証する。
func TestNewListSections(t *testing.T) {
	t.Parallel()

	newerEvent := &model.Event{ID: model.EventID(uuid.New()), Name: "ふわりす もちもちくじ"}
	olderEvent := &model.Event{ID: model.EventID(uuid.New()), Name: "こもりぐま ふわふわくじ"}
	newerCategory := &model.EventCategory{ID: model.EventCategoryID(uuid.New()), EventID: newerEvent.ID, Name: "B賞"}
	olderCategory := &model.EventCategory{ID: model.EventCategoryID(uuid.New()), EventID: olderEvent.ID, Name: "A賞"}
	bear := &model.Goods{ID: model.GoodsID(uuid.New()), EventCategoryID: newerCategory.ID, Name: "くまの子"}
	bird := &model.Goods{ID: model.GoodsID(uuid.New()), EventCategoryID: newerCategory.ID, Name: "とりの子"}
	rabbit := &model.Goods{ID: model.GoodsID(uuid.New()), EventCategoryID: olderCategory.ID, Name: "うさぎの子"}
	items := []*model.Item{
		{ID: model.ItemID(uuid.New()), GoodsID: bear.ID, Quantity: 2, Note: "未開封"},
		{ID: model.ItemID(uuid.New()), GoodsID: bird.ID, Quantity: 1},
		{ID: model.ItemID(uuid.New()), GoodsID: rabbit.ID, Quantity: 3},
	}

	sections := viewmodel.NewListSections(
		items,
		map[model.GoodsID]*model.Goods{bear.ID: bear, bird.ID: bird, rabbit.ID: rabbit},
		map[model.EventCategoryID]*model.EventCategory{newerCategory.ID: newerCategory, olderCategory.ID: olderCategory},
		map[model.EventID]*model.Event{newerEvent.ID: newerEvent, olderEvent.ID: olderEvent},
	)

	if len(sections) != 2 {
		t.Fatalf("欄の数 = %d、2 を期待", len(sections))
	}
	if sections[0].EventName != "ふわりす もちもちくじ" || sections[0].Quantity != 3 || len(sections[0].Items) != 2 {
		t.Errorf("1つめの欄 = %+v、ふわりす もちもちくじの2行・3点を期待", sections[0])
	}
	if sections[1].EventName != "こもりぐま ふわふわくじ" || sections[1].Quantity != 3 || len(sections[1].Items) != 1 {
		t.Errorf("2つめの欄 = %+v、こもりぐま ふわふわくじの1行・3点を期待", sections[1])
	}
	want := viewmodel.ListItemRow{ID: items[0].ID.String(), EventCategoryName: "B賞", GoodsName: "くまの子", Quantity: 2, Note: "未開封"}
	if sections[0].Items[0] != want {
		t.Errorf("1行め = %+v、%+v を期待", sections[0].Items[0], want)
	}

	if empty := viewmodel.NewListSections(nil, nil, nil, nil); len(empty) != 0 {
		t.Errorf("アイテムが無いときの欄 = %+v、空を期待", empty)
	}
}
