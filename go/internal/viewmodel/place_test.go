package viewmodel_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestNewPlacePrefectures は、公開中の駅と既に選んでいる駅を合わせて都道府県ごと・並び順にまとめ、
// チェックを入れた駅のある都道府県とそれ以外に分けることを検証する。
func TestNewPlacePrefectures(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	station := func(code model.PrefectureCode, name string, position int32) *model.Station {
		return &model.Station{ID: model.StationID(uuid.New()), PrefectureCode: code, Name: name, Position: position}
	}
	shibuya := station(13, "渋谷", 2)
	shinjuku := station(13, "新宿", 1)
	yokohama := station(14, "横浜", 1)
	archived := station(13, "閉じた駅", 3)
	archived.Status = model.MasterStatusArchived

	prefectures := viewmodel.NewPlacePrefectures(ctx, []*model.Station{yokohama, shibuya, shinjuku}, []*model.Station{shinjuku, archived}, []string{archived.ID.String()})

	if len(prefectures.Chosen) != 1 || prefectures.Chosen[0].Name != "東京都" || prefectures.Chosen[0].Code != "13" {
		t.Fatalf("Chosen = %+v、東京都だけを期待", prefectures.Chosen)
	}
	want := []viewmodel.PlaceStationOption{
		{ID: shinjuku.ID.String(), Name: "新宿"},
		{ID: shibuya.ID.String(), Name: "渋谷"},
		{ID: archived.ID.String(), Name: "閉じた駅", Checked: true},
	}
	if got := prefectures.Chosen[0].Stations; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("東京都の駅 = %+v、期待値 = %+v", got, want)
	}
	if len(prefectures.Others) != 1 || prefectures.Others[0].Name != "神奈川県" || prefectures.Others[0].Stations[0].Checked {
		t.Errorf("Others = %+v、チェックの無い神奈川県だけを期待", prefectures.Others)
	}
}

// TestPlaceSummary は、選んだ駅の名前を並んだ順に「・」でつなぎ、駅が無ければ空にすることを検証する。
func TestPlaceSummary(t *testing.T) {
	t.Parallel()

	if got := viewmodel.PlaceSummary([]*model.Station{{Name: "新宿"}, {Name: "川崎"}}); got != "新宿・川崎" {
		t.Errorf("PlaceSummary() = %q、期待値 = %q", got, "新宿・川崎")
	}
	if got := viewmodel.PlaceSummary(nil); got != "" {
		t.Errorf("空のPlaceSummary() = %q、空を期待", got)
	}
}

// TestNewPlaceGroups は、都道府県の順に並んだ駅を都道府県ごとにまとめ、駅の名前を「・」でつなぐことと、駅が無ければnilにすることを検証する。
func TestNewPlaceGroups(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	groups := viewmodel.NewPlaceGroups(ctx, []*model.Station{
		{PrefectureCode: 13, Name: "新宿"},
		{PrefectureCode: 13, Name: "渋谷"},
		{PrefectureCode: 14, Name: "川崎"},
	})

	want := []viewmodel.PlaceGroup{{PrefectureName: "東京都", StationNames: "新宿・渋谷"}, {PrefectureName: "神奈川県", StationNames: "川崎"}}
	if len(groups) != len(want) || groups[0] != want[0] || groups[1] != want[1] {
		t.Errorf("NewPlaceGroups() = %+v、期待値 = %+v", groups, want)
	}
	if got := viewmodel.NewPlaceGroups(ctx, nil); got != nil {
		t.Errorf("空のNewPlaceGroups() = %+v、nilを期待", got)
	}
}
