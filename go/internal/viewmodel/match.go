package viewmodel

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
)

// MatchItemRow は交換できるアイテムの1行。
type MatchItemRow struct {
	EventCategoryName string
	GoodsName         string
	// Quantity は交換できる数量。
	Quantity int32
}

// MatchItemRows は、もらえるもの・渡せるものの一方の、交換できるアイテムの行と数量の合計。
type MatchItemRows struct {
	// Quantity は Items の交換できる数量の合計。
	Quantity int64
	Items    []MatchItemRow
}

// newMatchItemRows は交換できるアイテムを行にする。goods・categories には items のマスタをすべて入れて渡す。
// 無いマスタの名前は空にする。
func newMatchItemRows(items []model.MatchItem, goods map[model.GoodsID]*model.Goods, categories map[model.EventCategoryID]*model.EventCategory) MatchItemRows {
	rows := MatchItemRows{Items: make([]MatchItemRow, len(items))}
	for i, item := range items {
		row := MatchItemRow{Quantity: item.Quantity}
		if g := goods[item.Item.GoodsID]; g != nil {
			row.GoodsName = g.Name
			if category := categories[g.EventCategoryID]; category != nil {
				row.EventCategoryName = category.Name
			}
		}
		rows.Quantity += int64(item.Quantity)
		rows.Items[i] = row
	}

	return rows
}

// MatchRows は、ユーザーと相手の間で交換できるアイテムを、もらえるもの・渡せるものに分けて行にしたもの。
type MatchRows struct {
	Receivable MatchItemRows
	Givable    MatchItemRows
}

// NewMatchRows は、ユーザーと相手の間で交換できるアイテムを行にする。
// goods・categories には match のアイテムのマスタをすべて入れて渡す。
func NewMatchRows(match model.Match, goods map[model.GoodsID]*model.Goods, categories map[model.EventCategoryID]*model.EventCategory) MatchRows {
	return MatchRows{
		Receivable: newMatchItemRows(match.Receivable, goods, categories),
		Givable:    newMatchItemRows(match.Givable, goods, categories),
	}
}

// MatchCandidate はマッチ候補の一覧の1人。
type MatchCandidate struct {
	Atname string
	// Places は候補の交換場所を都道府県ごとにまとめたもの。
	Places []PlaceGroup
	MatchRows
}

// NewMatchCandidates はマッチ候補を、候補の並び順のまま一覧の行にする。
// matches・stations は候補ごとの交換できるアイテムと交換場所で、goods・categories には交換できるアイテムのマスタをすべて入れて渡す。
func NewMatchCandidates(
	ctx context.Context,
	candidates []*model.User,
	matches map[model.UserID]model.Match,
	stations map[model.UserID][]*model.Station,
	goods map[model.GoodsID]*model.Goods,
	categories map[model.EventCategoryID]*model.EventCategory,
) []MatchCandidate {
	rows := make([]MatchCandidate, len(candidates))
	for i, candidate := range candidates {
		rows[i] = MatchCandidate{
			Atname:    candidate.Atname,
			Places:    NewPlaceGroups(ctx, stations[candidate.ID]),
			MatchRows: NewMatchRows(matches[candidate.ID], goods, categories),
		}
	}

	return rows
}
