package viewmodel

import (
	"strconv"

	"github.com/cutreapp/cutre/go/internal/model"
)

// ItemForm はリストに追加するフォームに入れる値。
// 数量は入力欄の値の文字列で持ち、エラーで描き直すときは送られた値をそのまま戻す。
type ItemForm struct {
	Kind     string
	Quantity string
	Note     string
	// LockVersion は編集画面を開いたときのアイテムの版。追加画面では使わない。
	LockVersion int32
}

// NewItemForm はリストに追加する画面を開いたときのフォームの値を返す。
// リストは入口 (カテゴリーのグッズの一覧) で押したほうを選んでおき、数量は1から始める。
func NewItemForm(kind string) ItemForm {
	return ItemForm{Kind: kind, Quantity: "1"}
}

// ItemKindLabelKey はリスト kind の名前 (「譲れる」「ほしい」) の翻訳キーを返す。
func ItemKindLabelKey(kind model.ItemKind) string {
	if kind == model.ItemKindWant {
		return "item_kind_want"
	}

	return "item_kind_give"
}

// NewItemEditForm はリストのアイテムの編集の画面を開いたときのフォームの値を、保存済みのアイテムから作る。
func NewItemEditForm(item *model.Item) ItemForm {
	return ItemForm{Kind: string(item.Kind), Quantity: strconv.Itoa(int(item.Quantity)), Note: item.Note, LockVersion: item.LockVersion}
}

// ListSection はユーザー向けのリストの、イベントごとの欄。
type ListSection struct {
	EventName string
	// Quantity は欄のアイテムの数量の合計。
	Quantity int64
	Items    []ListItemRow
}

// ListItemRow はユーザー向けのリストの、アイテムの1行。
type ListItemRow struct {
	ID                string
	EventCategoryName string
	GoodsName         string
	Quantity          int32
	// Note はアイテムのひとこと。無ければ空文字。
	Note string
}

// NewListSections はリストのアイテムを、イベントごとの欄にまとめる。
//
// items はイベントごとにまとまった順 (GetListUsecase の並び) で渡し、欄もその順に並べる。
// goods・categories・events には items のマスタをすべて入れて渡す。無いマスタの名前は空にする。
func NewListSections(items []*model.Item, goods map[model.GoodsID]*model.Goods, categories map[model.EventCategoryID]*model.EventCategory, events map[model.EventID]*model.Event) []ListSection {
	var sections []ListSection
	var currentEventID model.EventID
	for _, item := range items {
		row := ListItemRow{ID: item.ID.String(), Quantity: item.Quantity, Note: item.Note}
		var eventID model.EventID
		var eventName string
		if g := goods[item.GoodsID]; g != nil {
			row.GoodsName = g.Name
			if category := categories[g.EventCategoryID]; category != nil {
				row.EventCategoryName = category.Name
				eventID = category.EventID
				if event := events[category.EventID]; event != nil {
					eventName = event.Name
				}
			}
		}

		if len(sections) == 0 || eventID != currentEventID {
			sections = append(sections, ListSection{EventName: eventName})
			currentEventID = eventID
		}
		section := &sections[len(sections)-1]
		section.Quantity += int64(item.Quantity)
		section.Items = append(section.Items, row)
	}

	return sections
}
