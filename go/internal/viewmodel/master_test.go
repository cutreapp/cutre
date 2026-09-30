package viewmodel_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestNewEventCategoryCreateForm は、作成のフォームの並び順を、既存のカテゴリーの最後の値に100を足した値にし、
// カテゴリーが無ければ100にし、上限を超えるときは上限の値を返すことを検証する。
func TestNewEventCategoryCreateForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		categories []*model.EventCategory
		want       string
	}{
		{name: "カテゴリーが無い", categories: nil, want: "100"},
		{name: "カテゴリーがある", categories: []*model.EventCategory{{Position: 0}, {Position: 10}}, want: "110"},
		{name: "足すと上限ちょうど", categories: []*model.EventCategory{{Position: 9899}}, want: "9999"},
		{name: "足すと上限を超える", categories: []*model.EventCategory{{Position: 9950}}, want: "9999"},
		{name: "上限に達している", categories: []*model.EventCategory{{Position: 9999}}, want: "9999"},
	}
	for _, tt := range tests {
		if got := viewmodel.NewEventCategoryCreateForm(tt.categories).Position; got != tt.want {
			t.Errorf("%s: 並び順 = %q、期待値 = %q", tt.name, got, tt.want)
		}
	}
}

// TestNewGoodsCreateForm は、作成のフォームの並び順を、既存のグッズの最後の値に100を足した値にし、
// グッズが無ければ100にし、上限を超えるときは上限の値を返すことを検証する。
func TestNewGoodsCreateForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goods []*model.Goods
		want  string
	}{
		{name: "グッズが無い", goods: nil, want: "100"},
		{name: "グッズがある", goods: []*model.Goods{{Position: 3}}, want: "103"},
		{name: "足すと上限を超える", goods: []*model.Goods{{Position: 9900}}, want: "9999"},
		{name: "上限に達している", goods: []*model.Goods{{Position: 9999}}, want: "9999"},
	}
	for _, tt := range tests {
		if got := viewmodel.NewGoodsCreateForm(tt.goods).Position; got != tt.want {
			t.Errorf("%s: 並び順 = %q、期待値 = %q", tt.name, got, tt.want)
		}
	}
}

// TestNewAdminEventCategoryRows は、カテゴリーをID・名前・アーカイブしたかの行にすることを検証する。
func TestNewAdminEventCategoryRows(t *testing.T) {
	t.Parallel()

	rows := viewmodel.NewAdminEventCategoryRows([]*model.EventCategory{
		{Name: "A賞", Status: model.MasterStatusPublished},
		{Name: "B賞", Status: model.MasterStatusArchived},
	})
	if len(rows) != 2 || rows[0].Name != "A賞" || rows[0].Archived || rows[1].Name != "B賞" || !rows[1].Archived {
		t.Errorf("行 = %+v、A賞 (公開中)・B賞 (アーカイブ) を期待", rows)
	}
}
