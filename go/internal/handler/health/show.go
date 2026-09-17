package health

import (
	"encoding/json"
	"net/http"
)

// Show GET /health - サーバーが稼働中であることを示すJSONを返す。
//
// WriteHeader を呼んだ後はステータスコードが確定し、後続の http.Error が効かなくなる。
// エンコードに失敗したときにも500を返せるよう、ヘッダーを書く前にボディをmarshalする。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	body, err := json.Marshal(map[string]string{
		"status": "ok",
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
