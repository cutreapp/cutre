package viewmodel

import "github.com/cutreapp/cutre/go/internal/model"

// EventCategoryRow はユーザー向けのイベントのカテゴリーの一覧の1行。
type EventCategoryRow struct {
	ID   string
	Name string
	// GoodsCount はカテゴリーの公開中のグッズの種類の数。
	GoodsCount int64
	// Quantities はカテゴリーの中で、ユーザーがリストに入れたアイテムの数量の合計。
	Quantities model.ItemQuantities
}

// NewEventCategoryRows はカテゴリーをユーザー向けの一覧の行にする。
// goodsCounts と quantities に無いカテゴリーは、グッズやアイテムが無いものとして0にする。
func NewEventCategoryRows(categories []*model.EventCategory, goodsCounts map[model.EventCategoryID]int64, quantities map[model.EventCategoryID]model.ItemQuantities) []EventCategoryRow {
	rows := make([]EventCategoryRow, len(categories))
	for i, category := range categories {
		rows[i] = EventCategoryRow{
			ID:         category.ID.String(),
			Name:       category.Name,
			GoodsCount: goodsCounts[category.ID],
			Quantities: quantities[category.ID],
		}
	}

	return rows
}

// GoodsRow はユーザー向けのカテゴリーのグッズの一覧の1つ。
type GoodsRow struct {
	ID   string
	Name string
	// Give と Want は、ユーザーがそれぞれのリストに入れたアイテム。リストに無ければnil。
	Give *model.Item
	Want *model.Item
}

// NewGoodsRows はグッズを、ユーザーのリストにあるアイテムと組み合わせてユーザー向けの一覧の行にする。
// items にはリストにあるアイテムだけを渡す。同じグッズを同じリストに2つ入れることはできないため、グッズとリストごとに多くて1つになる。
func NewGoodsRows(goods []*model.Goods, items []*model.Item) []GoodsRow {
	listed := make(map[model.GoodsID]map[model.ItemKind]*model.Item, len(items))
	for _, item := range items {
		if listed[item.GoodsID] == nil {
			listed[item.GoodsID] = map[model.ItemKind]*model.Item{}
		}
		listed[item.GoodsID][item.Kind] = item
	}

	rows := make([]GoodsRow, len(goods))
	for i, g := range goods {
		rows[i] = GoodsRow{
			ID:   g.ID.String(),
			Name: g.Name,
			Give: listed[g.ID][model.ItemKindGive],
			Want: listed[g.ID][model.ItemKindWant],
		}
	}

	return rows
}
