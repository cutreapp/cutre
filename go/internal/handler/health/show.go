package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ヘルスチェックの応答に載せる状態。
const (
	statusOK          = "ok"
	statusUnavailable = "unavailable"
)

// Show GET /health - サーバーが稼働中で、データベースに届くことを示すJSONを返す。
// データベースに届かなければ503を返し、外形監視とデプロイ後の確認に知らせる。
//
// WriteHeader を呼んだ後はステータスコードが確定し、後続の http.Error が効かなくなる。
// エンコードに失敗したときにも500を返せるよう、ヘッダーを書く前にボディをmarshalする。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	// 現在の稼働状態を毎回確認させ、過去の正常・障害の応答を再利用させない。
	w.Header().Set("Cache-Control", "no-store")

	ctx := r.Context()

	result := h.checkHealthUC.Execute(ctx)

	status, code := statusOK, http.StatusOK
	if !result.DBHealthy {
		// 外形監視が障害の間も繰り返し叩くため、Error (Sentryへ送る) ではなくWarnで残す。
		slog.WarnContext(ctx, "ヘルスチェックでデータベースに届きませんでした", "error", result.DBError)
		status, code = statusUnavailable, http.StatusServiceUnavailable
		w.Header().Set("Retry-After", "5")
	}

	// 原因はログにだけ残し、公開の応答には内部の情報を載せない。
	body, err := json.Marshal(map[string]string{
		"status":   status,
		"database": status,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(body)
}
