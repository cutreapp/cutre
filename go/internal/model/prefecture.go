package model

import (
	"strconv"
	"strings"
)

// PrefectureCode はJIS X 0401の都道府県コード (1〜47)。列の型 (smallint) に合わせてint16で持つ。
type PrefectureCode int16

// Prefecture は都道府県。都道府県は増減しないため、テーブルにせずこのパッケージの定数で持つ。
type Prefecture struct {
	Code PrefectureCode
	// NameKey は都道府県の名前の翻訳キー。
	NameKey string
}

// prefectures はすべての都道府県を、都道府県コードの順に並べたもの。
var prefectures = []Prefecture{
	{Code: 1, NameKey: "prefecture_hokkaido"},
	{Code: 2, NameKey: "prefecture_aomori"},
	{Code: 3, NameKey: "prefecture_iwate"},
	{Code: 4, NameKey: "prefecture_miyagi"},
	{Code: 5, NameKey: "prefecture_akita"},
	{Code: 6, NameKey: "prefecture_yamagata"},
	{Code: 7, NameKey: "prefecture_fukushima"},
	{Code: 8, NameKey: "prefecture_ibaraki"},
	{Code: 9, NameKey: "prefecture_tochigi"},
	{Code: 10, NameKey: "prefecture_gunma"},
	{Code: 11, NameKey: "prefecture_saitama"},
	{Code: 12, NameKey: "prefecture_chiba"},
	{Code: 13, NameKey: "prefecture_tokyo"},
	{Code: 14, NameKey: "prefecture_kanagawa"},
	{Code: 15, NameKey: "prefecture_niigata"},
	{Code: 16, NameKey: "prefecture_toyama"},
	{Code: 17, NameKey: "prefecture_ishikawa"},
	{Code: 18, NameKey: "prefecture_fukui"},
	{Code: 19, NameKey: "prefecture_yamanashi"},
	{Code: 20, NameKey: "prefecture_nagano"},
	{Code: 21, NameKey: "prefecture_gifu"},
	{Code: 22, NameKey: "prefecture_shizuoka"},
	{Code: 23, NameKey: "prefecture_aichi"},
	{Code: 24, NameKey: "prefecture_mie"},
	{Code: 25, NameKey: "prefecture_shiga"},
	{Code: 26, NameKey: "prefecture_kyoto"},
	{Code: 27, NameKey: "prefecture_osaka"},
	{Code: 28, NameKey: "prefecture_hyogo"},
	{Code: 29, NameKey: "prefecture_nara"},
	{Code: 30, NameKey: "prefecture_wakayama"},
	{Code: 31, NameKey: "prefecture_tottori"},
	{Code: 32, NameKey: "prefecture_shimane"},
	{Code: 33, NameKey: "prefecture_okayama"},
	{Code: 34, NameKey: "prefecture_hiroshima"},
	{Code: 35, NameKey: "prefecture_yamaguchi"},
	{Code: 36, NameKey: "prefecture_tokushima"},
	{Code: 37, NameKey: "prefecture_kagawa"},
	{Code: 38, NameKey: "prefecture_ehime"},
	{Code: 39, NameKey: "prefecture_kochi"},
	{Code: 40, NameKey: "prefecture_fukuoka"},
	{Code: 41, NameKey: "prefecture_saga"},
	{Code: 42, NameKey: "prefecture_nagasaki"},
	{Code: 43, NameKey: "prefecture_kumamoto"},
	{Code: 44, NameKey: "prefecture_oita"},
	{Code: 45, NameKey: "prefecture_miyazaki"},
	{Code: 46, NameKey: "prefecture_kagoshima"},
	{Code: 47, NameKey: "prefecture_okinawa"},
}

// Prefectures はすべての都道府県を、都道府県コードの順に返す。
// 呼び出し側が並べ替えても定数が変わらないよう、複製を返す。
func Prefectures() []Prefecture {
	return append([]Prefecture(nil), prefectures...)
}

// FindPrefecture は都道府県コードの都道府県を返す。1〜47でないときはfalseを返す。
func FindPrefecture(code PrefectureCode) (Prefecture, bool) {
	if code < 1 || int(code) > len(prefectures) {
		return Prefecture{}, false
	}

	return prefectures[code-1], true
}

// ParsePrefectureCode はフォームやクエリの値を都道府県コードとして読む。
// 1〜47の整数でないときはfalseを返す。
func ParsePrefectureCode(value string) (PrefectureCode, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 16)
	if err != nil {
		return 0, false
	}
	prefecture, ok := FindPrefecture(PrefectureCode(parsed))
	if !ok {
		return 0, false
	}

	return prefecture.Code, true
}
