package model_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestParseItemIDs は、アイテムのIDの文字列を重複を除いて送られた順に読み、読めない値があればfalseを返すことを検証する。
func TestParseItemIDs(t *testing.T) {
	t.Parallel()

	a, b := uuid.New(), uuid.New()

	ids, ok := model.ParseItemIDs([]string{b.String(), a.String(), b.String()})
	if !ok || len(ids) != 2 || ids[0] != model.ItemID(b) || ids[1] != model.ItemID(a) {
		t.Errorf("ParseItemIDs() = (%v, %v)、(%s, %s) とtrueを期待", ids, ok, b, a)
	}

	if ids, ok := model.ParseItemIDs(nil); !ok || len(ids) != 0 {
		t.Errorf("ParseItemIDs(nil) = (%v, %v)、空とtrueを期待", ids, ok)
	}

	if _, ok := model.ParseItemIDs([]string{a.String(), "not-a-uuid"}); ok {
		t.Error("読めない値を含むのに、ParseItemIDs() がtrueを返した")
	}
}
