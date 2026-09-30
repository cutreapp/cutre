package model_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestPrefectures は、47の都道府県をコードの1から順に持ち、翻訳キーが重ならないことを検証する。
func TestPrefectures(t *testing.T) {
	t.Parallel()

	prefectures := model.Prefectures()
	if len(prefectures) != 47 {
		t.Fatalf("都道府県の数 = %d、期待値 = 47", len(prefectures))
	}
	keys := map[string]bool{}
	for i, prefecture := range prefectures {
		if prefecture.Code != model.PrefectureCode(i+1) {
			t.Errorf("%d番目のコード = %d、期待値 = %d", i+1, prefecture.Code, i+1)
		}
		if keys[prefecture.NameKey] {
			t.Errorf("翻訳キー %q が重なっている", prefecture.NameKey)
		}
		keys[prefecture.NameKey] = true
	}
}

// TestFindPrefecture は、1〜47のコードの都道府県を返し、範囲の外のコードにはfalseを返すことを検証する。
func TestFindPrefecture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code    model.PrefectureCode
		wantKey string
		wantOK  bool
	}{
		{code: 1, wantKey: "prefecture_hokkaido", wantOK: true},
		{code: 13, wantKey: "prefecture_tokyo", wantOK: true},
		{code: 47, wantKey: "prefecture_okinawa", wantOK: true},
		{code: 0},
		{code: 48},
		{code: -1},
	}

	for _, tt := range tests {
		got, ok := model.FindPrefecture(tt.code)
		if ok != tt.wantOK || got.NameKey != tt.wantKey {
			t.Errorf("FindPrefecture(%d) = (%+v, %v)、期待値 = (%q, %v)", tt.code, got, ok, tt.wantKey, tt.wantOK)
		}
	}
}

// TestParsePrefectureCode は、1〜47の整数を都道府県コードとして読み、それ以外にはfalseを返すことを検証する。
func TestParsePrefectureCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value  string
		want   model.PrefectureCode
		wantOK bool
	}{
		{value: "1", want: 1, wantOK: true},
		{value: " 47 ", want: 47, wantOK: true},
		{value: ""},
		{value: "0"},
		{value: "48"},
		{value: "tokyo"},
	}

	for _, tt := range tests {
		got, ok := model.ParsePrefectureCode(tt.value)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("ParsePrefectureCode(%q) = (%d, %v)、期待値 = (%d, %v)", tt.value, got, ok, tt.want, tt.wantOK)
		}
	}
}
