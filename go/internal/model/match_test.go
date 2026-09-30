package model_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestNewMatch は、相手の譲れるアイテムのうち自分がほしいものをもらえるものに、自分の譲れるアイテムのうち相手がほしいものを渡せるものにし、
// 交換できる数量を、譲れる数量とほしい数量の小さいほうにすることを検証する。
// ほかのユーザーのアイテムと、リストから外したアイテムは数えない。
func TestNewMatch(t *testing.T) {
	t.Parallel()

	me := model.UserID(uuid.New())
	partner := model.UserID(uuid.New())
	other := model.UserID(uuid.New())
	goodsA, goodsB, goodsC, goodsD := model.GoodsID(uuid.New()), model.GoodsID(uuid.New()), model.GoodsID(uuid.New()), model.GoodsID(uuid.New())
	item := func(userID model.UserID, goodsID model.GoodsID, kind model.ItemKind, quantity int32) *model.Item {
		return &model.Item{ID: model.ItemID(uuid.New()), UserID: userID, GoodsID: goodsID, Kind: kind, Status: model.ItemStatusListed, Quantity: quantity}
	}

	partnerGiveA := item(partner, goodsA, model.ItemKindGive, 3)
	partnerGiveB := item(partner, goodsB, model.ItemKindGive, 1)
	myGiveC := item(me, goodsC, model.ItemKindGive, 1)
	removedWantD := item(partner, goodsD, model.ItemKindWant, 1)
	removedWantD.Status = model.ItemStatusRemoved
	items := []*model.Item{
		partnerGiveA,
		partnerGiveB,
		item(partner, goodsC, model.ItemKindWant, 2),
		removedWantD,
		myGiveC,
		item(me, goodsD, model.ItemKindGive, 1),
		item(me, goodsA, model.ItemKindWant, 2),
		// 自分がほしいグッズを、相手ではないユーザーがほしがっていても数えない。
		item(other, goodsB, model.ItemKindWant, 1),
	}

	match := model.NewMatch(me, partner, items)

	if len(match.Receivable) != 1 || match.Receivable[0].Item != partnerGiveA || match.Receivable[0].Quantity != 2 {
		t.Errorf("Receivable = %+v、相手の譲れるアイテムAを2点 (ほしい数量) を期待", match.Receivable)
	}
	if len(match.Givable) != 1 || match.Givable[0].Item != myGiveC || match.Givable[0].Quantity != 1 {
		t.Errorf("Givable = %+v、自分の譲れるアイテムCを1点 (譲れる数量) を期待", match.Givable)
	}
	if !match.IsTradable() {
		t.Error("IsTradable() = false、true を期待")
	}
	if got := match.ReceivableQuantity(); got != 2 {
		t.Errorf("ReceivableQuantity() = %d、期待値 = 2", got)
	}
	if got := match.GivableQuantity(); got != 1 {
		t.Errorf("GivableQuantity() = %d、期待値 = 1", got)
	}
}

// TestMatch_IsTradable は、もらえるものと渡せるもののどちらかが無ければ交換を申し込めないとすることを検証する。
func TestMatch_IsTradable(t *testing.T) {
	t.Parallel()

	matchItem := model.MatchItem{Item: &model.Item{}, Quantity: 1}
	tests := []struct {
		name  string
		match model.Match
		want  bool
	}{
		{name: "両方ある", match: model.Match{Receivable: []model.MatchItem{matchItem}, Givable: []model.MatchItem{matchItem}}, want: true},
		{name: "もらえるものだけ", match: model.Match{Receivable: []model.MatchItem{matchItem}}, want: false},
		{name: "渡せるものだけ", match: model.Match{Givable: []model.MatchItem{matchItem}}, want: false},
		{name: "どちらも無い", match: model.Match{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.match.IsTradable(); got != tt.want {
				t.Errorf("IsTradable() = %v、期待値 = %v", got, tt.want)
			}
		})
	}
}

// TestMatch_Select は、交換できるアイテムのうち選んだものだけを元の順のまま残し、交換できるアイテムに無いIDは無視することを検証する。
func TestMatch_Select(t *testing.T) {
	t.Parallel()

	matchItem := func() model.MatchItem {
		return model.MatchItem{Item: &model.Item{ID: model.ItemID(uuid.New())}, Quantity: 1}
	}
	receivableA, receivableB, givableA := matchItem(), matchItem(), matchItem()
	match := model.Match{Receivable: []model.MatchItem{receivableA, receivableB}, Givable: []model.MatchItem{givableA}}

	selected := match.Select(
		[]model.ItemID{receivableB.Item.ID, receivableA.Item.ID, model.ItemID(uuid.New())},
		// もらえるもののIDを渡すものに選んでも、渡せるものには入れない。
		[]model.ItemID{receivableA.Item.ID},
	)

	if len(selected.Receivable) != 2 || selected.Receivable[0].Item != receivableA.Item || selected.Receivable[1].Item != receivableB.Item {
		t.Errorf("Receivable = %+v、もらえるものの2つを元の順で期待", selected.Receivable)
	}
	if len(selected.Givable) != 0 {
		t.Errorf("Givable = %+v、空を期待", selected.Givable)
	}
}
