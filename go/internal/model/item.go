package model

import (
	"time"

	"github.com/google/uuid"
)

// ItemKind はアイテムを入れたリスト。
type ItemKind string

const (
	// ItemKindGive は譲れるリスト。
	ItemKindGive ItemKind = "give"
	// ItemKindWant はほしいリスト。
	ItemKindWant ItemKind = "want"
)

// ItemKinds はすべてのリストを、画面に並べる順で返す。
// 呼び出しごとに新しいスライスを返すため、呼び出し側の変更が他へ波及しない。
func ItemKinds() []ItemKind {
	return []ItemKind{ItemKindGive, ItemKindWant}
}

// ParseItemKind はsが表す ItemKind を、表しているかどうかとともに返す。
// フォームやクエリの値が型に入る境界で使う。
func ParseItemKind(s string) (ItemKind, bool) {
	for _, kind := range ItemKinds() {
		if string(kind) == s {
			return kind, true
		}
	}

	return "", false
}

// ItemStatus はアイテムの状態。
type ItemStatus string

const (
	// ItemStatusListed はリストにある状態。一覧・件数・マッチはこの状態のアイテムだけを数える。
	ItemStatusListed ItemStatus = "listed"
	// ItemStatusRemoved はリストから外した状態。交換の品から辿れるよう、行は消さずに残す。
	ItemStatusRemoved ItemStatus = "removed"
)

// Item はユーザーが譲れる・ほしいのリストに入れたアイテム。
//
// マスタのグッズ (Goods) とは別のもので、「このユーザーが、このグッズを、この数量だけ譲れる (ほしい)」を表す。
// リストから外しても行は消さず、Status を ItemStatusRemoved にする。
// Quantity は譲れる数・ほしい数で、「交換できた」で減っていく。Note はひとことで、無ければ空文字。
// LockVersion は編集画面を開いたあとに変更されたかを検出する版。
type Item struct {
	ID          ItemID
	UserID      UserID
	GoodsID     GoodsID
	Kind        ItemKind
	Status      ItemStatus
	Quantity    int32
	Note        string
	LockVersion int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ItemQuantities はリストごとのアイテムの数量の合計。イベントやカテゴリーの中に、自分が何点入れているかを示すのに使う。
type ItemQuantities struct {
	Give int64
	Want int64
}

// Add は kind のリストの合計に quantity を足す。
func (q *ItemQuantities) Add(kind ItemKind, quantity int64) {
	switch kind {
	case ItemKindGive:
		q.Give += quantity
	case ItemKindWant:
		q.Want += quantity
	}
}

// ParseItemIDs は、フォームやクエリで送られたアイテムのIDの文字列を、重複を除いて送られた順の ItemID にする。
// 読めない値が1つでもあれば、それまでに読めたIDとfalseを返す。
func ParseItemIDs(raws []string) ([]ItemID, bool) {
	ids := make([]ItemID, 0, len(raws))
	seen := make(map[ItemID]bool, len(raws))
	for _, raw := range raws {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return ids, false
		}
		id := ItemID(parsed)
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	return ids, true
}
