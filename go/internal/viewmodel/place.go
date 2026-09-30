package viewmodel

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/cutreapp/cutre/go/internal/model"
)

// PlaceForm は交換場所のフォームに入れる値。エラーで描き直すときは送られた値をそのまま戻す。
type PlaceForm struct {
	// StationIDs はチェックを入れた駅のID。
	StationIDs []string
	PlaceNote  string
	// LockVersion は画面を開いたときの交換場所の版。
	LockVersion int32
}

// NewPlaceForm は保存済みの交換場所を、フォームに入れる値にする。
func NewPlaceForm(stations []*model.Station, placeNote string, lockVersion int32) PlaceForm {
	ids := make([]string, len(stations))
	for i, station := range stations {
		ids[i] = station.ID.String()
	}

	return PlaceForm{StationIDs: ids, PlaceNote: placeNote, LockVersion: lockVersion}
}

// PlaceStationOption は交換場所の画面で選ぶ駅の1つ。
type PlaceStationOption struct {
	ID      string
	Name    string
	Checked bool
}

// PlacePrefecture は交換場所の画面で、1つの都道府県の駅の選択肢をまとめたもの。
type PlacePrefecture struct {
	// Code は都道府県コードの数字。駅を都道府県ごとにまとめるときの区切りに使う。
	Code string
	// Name は表示中のロケールの都道府県の名前。
	Name     string
	Stations []PlaceStationOption
}

// PlacePrefectures は交換場所の画面の、都道府県ごとの駅の選択肢。
type PlacePrefectures struct {
	// Chosen は駅にチェックを入れた都道府県。交換場所として並べる。
	Chosen []PlacePrefecture
	// Others はまだ駅を選んでいない都道府県。「都道府県を追加」の中に畳んでおく。
	Others []PlacePrefecture
}

// NewPlacePrefectures は、交換場所の選択肢を都道府県ごとにまとめ、チェックを入れた駅のある都道府県とそれ以外に分ける。
//
// 選択肢は公開中の駅 published と、既に選んでいる駅 selected (アーカイブしたものを含む) を合わせたもので、
// 都道府県コードの順、都道府県の中では並び順に並べる。checkedIDs はチェックを入れる駅のIDで、
// 保存済みの交換場所か、エラーで描き直すときに送られた値を渡す。
func NewPlacePrefectures(ctx context.Context, published, selected []*model.Station, checkedIDs []string) PlacePrefectures {
	byID := make(map[model.StationID]*model.Station, len(published)+len(selected))
	for _, station := range slices.Concat(published, selected) {
		byID[station.ID] = station
	}
	stations := make([]*model.Station, 0, len(byID))
	for _, station := range byID {
		stations = append(stations, station)
	}
	slices.SortFunc(stations, func(a, b *model.Station) int {
		return cmp.Or(
			cmp.Compare(a.PrefectureCode, b.PrefectureCode),
			cmp.Compare(a.Position, b.Position),
			strings.Compare(a.ID.String(), b.ID.String()),
		)
	})

	checked := make(map[string]bool, len(checkedIDs))
	for _, id := range checkedIDs {
		checked[id] = true
	}

	var groups []PlacePrefecture
	for _, station := range stations {
		code := formatPrefectureCode(station.PrefectureCode)
		if len(groups) == 0 || groups[len(groups)-1].Code != code {
			groups = append(groups, PlacePrefecture{Code: code, Name: PrefectureName(ctx, station.PrefectureCode)})
		}
		id := station.ID.String()
		last := &groups[len(groups)-1]
		last.Stations = append(last.Stations, PlaceStationOption{ID: id, Name: station.Name, Checked: checked[id]})
	}

	var prefectures PlacePrefectures
	for _, group := range groups {
		if slices.ContainsFunc(group.Stations, func(option PlaceStationOption) bool { return option.Checked }) {
			prefectures.Chosen = append(prefectures.Chosen, group)
		} else {
			prefectures.Others = append(prefectures.Others, group)
		}
	}

	return prefectures
}

// PlaceSummary は交換場所に選んだ駅の名前を、並んだ順に「・」でつないだ要約にする。駅が無ければ空文字を返す。
func PlaceSummary(stations []*model.Station) string {
	names := make([]string, len(stations))
	for i, station := range stations {
		names[i] = station.Name
	}

	return strings.Join(names, "・")
}

// PlaceGroup は、ほかのユーザーの交換場所の、1つの都道府県の駅をまとめたもの。
type PlaceGroup struct {
	// PrefectureName は表示中のロケールの都道府県の名前。
	PrefectureName string
	// StationNames は都道府県の駅の名前を、並んだ順に「・」でつないだもの。
	StationNames string
}

// NewPlaceGroups は、都道府県コードの順、都道府県の中では並び順に並んだ交換場所の駅を、都道府県ごとにまとめる。
// 駅が無ければnilを返す。
func NewPlaceGroups(ctx context.Context, stations []*model.Station) []PlaceGroup {
	var groups []PlaceGroup
	var names []string
	for i, station := range stations {
		names = append(names, station.Name)
		if i+1 == len(stations) || stations[i+1].PrefectureCode != station.PrefectureCode {
			groups = append(groups, PlaceGroup{PrefectureName: PrefectureName(ctx, station.PrefectureCode), StationNames: strings.Join(names, "・")})
			names = nil
		}
	}

	return groups
}
