package viewmodel_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestNewAdminStationGroups は、並んだ駅を都道府県ごとにまとめ、都道府県の名前を表示中のロケールで引くことを検証する。
func TestNewAdminStationGroups(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	stations := []*model.Station{
		{PrefectureCode: 13, Name: "新宿"},
		{PrefectureCode: 13, Name: "渋谷", Status: model.MasterStatusArchived},
		{PrefectureCode: 27, Name: "梅田"},
	}

	groups := viewmodel.NewAdminStationGroups(ctx, stations)
	if len(groups) != 2 {
		t.Fatalf("都道府県の数 = %d、期待値 = 2", len(groups))
	}
	if groups[0].PrefectureCode != "13" || groups[0].PrefectureName != "東京都" || len(groups[0].Stations) != 2 || !groups[0].Stations[1].Archived {
		t.Errorf("1つ目の都道府県 = %+v、東京都の2駅 (2つ目はアーカイブ) を期待", groups[0])
	}
	if groups[1].PrefectureCode != "27" || groups[1].PrefectureName != "大阪府" || len(groups[1].Stations) != 1 {
		t.Errorf("2つ目の都道府県 = %+v、大阪府の1駅を期待", groups[1])
	}

	if got := viewmodel.NewAdminStationGroups(ctx, nil); len(got) != 0 {
		t.Errorf("駅が無いときの都道府県 = %+v、空を期待", got)
	}
}

// TestNewStationCreateForm は、都道府県が決まっていればそれを選び、並び順をその都道府県の駅の最後の値に100を足した値にし、
// 決まっていなければ都道府県を空・並び順を100にすることを検証する。
func TestNewStationCreateForm(t *testing.T) {
	t.Parallel()

	stations := []*model.Station{{PrefectureCode: 13, Position: 3}, {PrefectureCode: 13, Position: 8}, {PrefectureCode: 27, Position: 20}}
	tests := []struct {
		name         string
		code         model.PrefectureCode
		ok           bool
		wantCode     string
		wantPosition string
	}{
		{name: "駅のある都道府県", code: 13, ok: true, wantCode: "13", wantPosition: "108"},
		{name: "駅の無い都道府県", code: 1, ok: true, wantCode: "1", wantPosition: "100"},
		{name: "都道府県が決まっていない", wantCode: "", wantPosition: "100"},
	}

	for _, tt := range tests {
		got := viewmodel.NewStationCreateForm(stations, tt.code, tt.ok)
		if got.PrefectureCode != tt.wantCode || got.Position != tt.wantPosition {
			t.Errorf("%s: フォーム = %+v、都道府県 %q・並び順 %q を期待", tt.name, got, tt.wantCode, tt.wantPosition)
		}
	}
}

// TestPrefectureName は、都道府県の名前を表示中のロケールで返し、都道府県コードでない値は数字のまま返すことを検証する。
func TestPrefectureName(t *testing.T) {
	t.Parallel()

	if got := viewmodel.PrefectureName(i18n.SetLocale(context.Background(), i18n.LangEn), 1); got != "Hokkaido" {
		t.Errorf("英語の北海道 = %q、期待値 = %q", got, "Hokkaido")
	}
	if got := viewmodel.PrefectureName(i18n.SetLocale(context.Background(), i18n.LangJa), 99); got != "99" {
		t.Errorf("都道府県コードでない値 = %q、期待値 = %q", got, "99")
	}
}
