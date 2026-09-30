package viewmodel_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestNewMatchCandidates は、マッチ候補を候補の並び順のまま行にし、交換場所を都道府県ごとにまとめ、
// もらえるもの・渡せるものにカテゴリーとグッズの名前と数量の合計を添えることを検証する。無いマスタの名前は空にする。
func TestNewMatchCandidates(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	category := &model.EventCategory{ID: model.EventCategoryID(uuid.New()), Name: "B賞"}
	goods := &model.Goods{ID: model.GoodsID(uuid.New()), EventCategoryID: category.ID, Name: "くまの子"}
	first := &model.User{ID: model.UserID(uuid.New()), Atname: "yuzu"}
	second := &model.User{ID: model.UserID(uuid.New()), Atname: "minami"}
	matches := map[model.UserID]model.Match{
		first.ID: {
			Receivable: []model.MatchItem{{Item: &model.Item{GoodsID: goods.ID}, Quantity: 2}},
			Givable:    []model.MatchItem{{Item: &model.Item{GoodsID: model.GoodsID(uuid.New())}, Quantity: 1}},
		},
	}
	stations := map[model.UserID][]*model.Station{first.ID: {{PrefectureCode: 13, Name: "渋谷"}}}

	rows := viewmodel.NewMatchCandidates(ctx, []*model.User{first, second}, matches, stations, map[model.GoodsID]*model.Goods{goods.ID: goods}, map[model.EventCategoryID]*model.EventCategory{category.ID: category})

	if len(rows) != 2 || rows[0].Atname != "yuzu" || rows[1].Atname != "minami" {
		t.Fatalf("NewMatchCandidates() = %+v、yuzu・minami の順を期待", rows)
	}
	if got := rows[0].Places; len(got) != 1 || got[0] != (viewmodel.PlaceGroup{PrefectureName: "東京都", StationNames: "渋谷"}) {
		t.Errorf("Places = %+v、東京都の渋谷を期待", got)
	}
	wantReceivable := viewmodel.MatchItemRows{Quantity: 2, Items: []viewmodel.MatchItemRow{{EventCategoryName: "B賞", GoodsName: "くまの子", Quantity: 2}}}
	if got := rows[0].Receivable; got.Quantity != wantReceivable.Quantity || len(got.Items) != 1 || got.Items[0] != wantReceivable.Items[0] {
		t.Errorf("Receivable = %+v、期待値 = %+v", got, wantReceivable)
	}
	if got := rows[0].Givable; got.Quantity != 1 || len(got.Items) != 1 || got.Items[0] != (viewmodel.MatchItemRow{Quantity: 1}) {
		t.Errorf("Givable = %+v、名前の無い1点を期待", got)
	}
	if got := rows[1]; len(got.Places) != 0 || len(got.Receivable.Items) != 0 || len(got.Givable.Items) != 0 {
		t.Errorf("2人目 = %+v、交換場所も交換できるものも無しを期待", got)
	}
}
