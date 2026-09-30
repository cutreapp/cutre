package model

// Match は、ユーザー (自分) とほかのユーザー (相手) の間で交換できるアイテム。
//
// 交換できるのは、渡す人の譲れるリストと、受け取る人のほしいリストに同じグッズがあるアイテム。
// マッチ候補 (交換場所の都道府県が同じで、もらえるものと渡せるものがどちらもある相手) の一覧と、
// 相手のプロフィールの「あなたとの交換」に使う。
type Match struct {
	// Receivable は自分がもらえるもの。相手の譲れるアイテムのうち、自分のほしいリストに同じグッズがあるもの。
	Receivable []MatchItem
	// Givable は自分が渡せるもの。自分の譲れるアイテムのうち、相手のほしいリストに同じグッズがあるもの。
	Givable []MatchItem
}

// MatchItem は交換できるアイテムの1つ。
type MatchItem struct {
	// Item は渡す人の譲れるアイテム。
	Item *Item
	// Quantity は交換できる数量。渡す人の譲れる数量と、受け取る人のほしい数量の小さいほう。
	Quantity int32
}

// NewMatch は、ユーザー userID と相手 partnerID のリストにあるアイテム items から、2人の間で交換できるアイテムを求める。
//
// items には2人のアイテムを混ぜて渡してよく、ほかのユーザーのアイテムとリストから外したアイテムは無視する。
// Receivable と Givable は、items の中の譲れるアイテムの順に並べる。
func NewMatch(userID, partnerID UserID, items []*Item) Match {
	wants := map[UserID]map[GoodsID]int32{userID: {}, partnerID: {}}
	for _, item := range items {
		if item.Status == ItemStatusListed && item.Kind == ItemKindWant && wants[item.UserID] != nil {
			wants[item.UserID][item.GoodsID] += item.Quantity
		}
	}

	var match Match
	for _, item := range items {
		if item.Status != ItemStatusListed || item.Kind != ItemKindGive {
			continue
		}
		switch item.UserID {
		case partnerID:
			if want := wants[userID][item.GoodsID]; want > 0 {
				match.Receivable = append(match.Receivable, MatchItem{Item: item, Quantity: min(item.Quantity, want)})
			}
		case userID:
			if want := wants[partnerID][item.GoodsID]; want > 0 {
				match.Givable = append(match.Givable, MatchItem{Item: item, Quantity: min(item.Quantity, want)})
			}
		}
	}

	return match
}

// IsTradable は、もらえるものと渡せるものがどちらも1つ以上あり、交換を申し込めるかを返す。
func (m Match) IsTradable() bool {
	return len(m.Receivable) > 0 && len(m.Givable) > 0
}

// ReceivableQuantity は、もらえるものの交換できる数量の合計を返す。
func (m Match) ReceivableQuantity() int64 {
	return sumMatchItemQuantities(m.Receivable)
}

// GivableQuantity は、渡せるものの交換できる数量の合計を返す。
func (m Match) GivableQuantity() int64 {
	return sumMatchItemQuantities(m.Givable)
}

// sumMatchItemQuantities は交換できるアイテムの数量の合計を返す。
func sumMatchItemQuantities(items []MatchItem) int64 {
	var sum int64
	for _, item := range items {
		sum += int64(item.Quantity)
	}

	return sum
}

// Select は、交換できるアイテムのうち、もらうものとして選んだ receiveIDs と、渡すものとして選んだ giveIDs に入っているものだけを残す。
// 交換を申し込む組み合わせを、交換できるアイテムの中から選ぶのに使う。交換できるアイテムに無いIDは無視する。
func (m Match) Select(receiveIDs, giveIDs []ItemID) Match {
	return Match{
		Receivable: selectMatchItems(m.Receivable, receiveIDs),
		Givable:    selectMatchItems(m.Givable, giveIDs),
	}
}

// selectMatchItems は items のうち、ids に入っているアイテムだけを items の順のまま返す。
func selectMatchItems(items []MatchItem, ids []ItemID) []MatchItem {
	selected := make(map[ItemID]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}

	var result []MatchItem
	for _, item := range items {
		if selected[item.Item.ID] {
			result = append(result, item)
		}
	}

	return result
}
