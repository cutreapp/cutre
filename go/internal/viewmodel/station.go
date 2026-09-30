package viewmodel

import (
	"context"
	"strconv"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// PrefectureName は都道府県コードの都道府県の名前を、表示中のロケールで返す。
// 都道府県コードでない値は、データの誤りに気付けるようコードの数字をそのまま返す。
func PrefectureName(ctx context.Context, code model.PrefectureCode) string {
	prefecture, ok := model.FindPrefecture(code)
	if !ok {
		return strconv.Itoa(int(code))
	}

	return i18n.T(ctx, prefecture.NameKey)
}

// PrefectureOption は都道府県を選ぶ欄の選択肢の1つ。
type PrefectureOption struct {
	// Value は選択肢の値。都道府県コードの数字。
	Value string
	// Label は表示中のロケールの都道府県の名前。
	Label string
}

// NewPrefectureOptions はすべての都道府県を、都道府県コードの順に選択肢にする。
func NewPrefectureOptions(ctx context.Context) []PrefectureOption {
	prefectures := model.Prefectures()
	options := make([]PrefectureOption, len(prefectures))
	for i, prefecture := range prefectures {
		options[i] = PrefectureOption{Value: formatPrefectureCode(prefecture.Code), Label: i18n.T(ctx, prefecture.NameKey)}
	}

	return options
}

// AdminStationGroup は管理画面の駅の一覧で、1つの都道府県の駅をまとめたもの。
type AdminStationGroup struct {
	// PrefectureCode は都道府県コードの数字。作成の画面へ都道府県を渡すのと、見出しのidに使う。
	PrefectureCode string
	// PrefectureName は表示中のロケールの都道府県の名前。
	PrefectureName string
	// Stations は都道府県の駅を並び順に並べたもの。
	Stations []AdminMasterRow
}

// NewAdminStationGroups は、都道府県コードの順、都道府県の中では並び順に並んだ駅を、都道府県ごとにまとめる。
// 駅の無い都道府県は含めない。
func NewAdminStationGroups(ctx context.Context, stations []*model.Station) []AdminStationGroup {
	var groups []AdminStationGroup
	for _, station := range stations {
		code := formatPrefectureCode(station.PrefectureCode)
		if len(groups) == 0 || groups[len(groups)-1].PrefectureCode != code {
			groups = append(groups, AdminStationGroup{PrefectureCode: code, PrefectureName: PrefectureName(ctx, station.PrefectureCode)})
		}
		last := &groups[len(groups)-1]
		last.Stations = append(last.Stations, AdminMasterRow{ID: station.ID.String(), Name: station.Name, Archived: station.IsArchived()})
	}

	return groups
}

// StationForm は管理画面の駅の作成・編集のフォームに入れる値。
// 都道府県と並び順は入力欄の値の文字列で持ち、エラーで描き直すときは送られた値をそのまま戻す。
type StationForm struct {
	PrefectureCode string
	Name           string
	Position       string
	// LockVersion は編集のフォームが持ち回る版。作成のフォームでは使わない。
	LockVersion int32
}

// NewStationForm は保存済みの駅を、編集のフォームに入れる値にする。
func NewStationForm(station *model.Station) StationForm {
	return StationForm{
		PrefectureCode: formatPrefectureCode(station.PrefectureCode),
		Name:           station.Name,
		Position:       formatPosition(station.Position),
		LockVersion:    station.LockVersion,
	}
}

// NewStationCreateForm は駅の作成のフォームの初期値を返す。
//
// 都道府県 prefectureCode が決まっているとき (ok がtrue) は、その都道府県を選んでおき、並び順には
// その都道府県の既存の駅 (stations は都道府県コードの順、都道府県の中では並び順に並んだもの) の最後の値に100を足した値
// (駅が無ければ100) を入れる。決まっていないときは都道府県を空にし、並び順を100にする。
func NewStationCreateForm(stations []*model.Station, prefectureCode model.PrefectureCode, ok bool) StationForm {
	if !ok {
		return StationForm{Position: formatPosition(masterPositionStep)}
	}

	form := StationForm{PrefectureCode: formatPrefectureCode(prefectureCode), Position: formatPosition(masterPositionStep)}
	for _, station := range stations {
		if station.PrefectureCode == prefectureCode {
			form.Position = nextMasterPosition(station.Position)
		}
	}

	return form
}

// formatPrefectureCode は都道府県コードを選択肢とクエリの値にする。
func formatPrefectureCode(code model.PrefectureCode) string {
	return strconv.Itoa(int(code))
}
