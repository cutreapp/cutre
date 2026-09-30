package viewmodel_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestNewGoodsRows は、グッズを並び順のまま行にし、グッズごとに譲れる・ほしいのアイテムを振り分けることを検証する。
func TestNewGoodsRows(t *testing.T) {
	t.Parallel()

	first := &model.Goods{ID: model.GoodsID(uuid.New()), Name: "くまの子"}
	second := &model.Goods{ID: model.GoodsID(uuid.New()), Name: "ねこの子"}
	give := &model.Item{GoodsID: first.ID, Kind: model.ItemKindGive, Quantity: 2}
	want := &model.Item{GoodsID: first.ID, Kind: model.ItemKindWant, Quantity: 1}

	rows := viewmodel.NewGoodsRows([]*model.Goods{first, second}, []*model.Item{want, give})

	if len(rows) != 2 || rows[0].Name != "くまの子" || rows[1].Name != "ねこの子" {
		t.Fatalf("行 = %+v、[くまの子 ねこの子] を期待", rows)
	}
	if rows[0].Give != give || rows[0].Want != want {
		t.Errorf("くまの子のアイテム = (%+v, %+v)、譲れる・ほしいの両方を期待", rows[0].Give, rows[0].Want)
	}
	if rows[1].Give != nil || rows[1].Want != nil {
		t.Errorf("ねこの子のアイテム = (%+v, %+v)、どちらも無いことを期待", rows[1].Give, rows[1].Want)
	}
}
