package model_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestUserID_JSON は、UserIDをJSONではUUIDの文字列表記で書き出し、同じ表記から読み戻せることを検証する。
func TestUserID_JSON(t *testing.T) {
	t.Parallel()

	id := model.UserID(uuid.MustParse("0192f0c6-1234-7abc-8def-0123456789ab"))

	encoded, err := json.Marshal(struct {
		UserID model.UserID `json:"user_id"`
	}{UserID: id})
	if err != nil {
		t.Fatalf("json.Marshal()のエラー = %v", err)
	}
	if want := `{"user_id":"0192f0c6-1234-7abc-8def-0123456789ab"}`; string(encoded) != want {
		t.Errorf("JSON = %s、期待値 = %s", encoded, want)
	}

	var decoded struct {
		UserID model.UserID `json:"user_id"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal()のエラー = %v", err)
	}
	if decoded.UserID != id {
		t.Errorf("読み戻したUserID = %v、期待値 = %v", decoded.UserID, id)
	}
}
