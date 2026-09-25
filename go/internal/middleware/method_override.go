package middleware

import (
	"net/http"
	"strings"
)

// MethodOverrideFieldName はメソッドの上書きに使うフォームフィールドの名前。
const MethodOverrideFieldName = "_method"

// MethodOverride はPOSTリクエストのメソッドを、_method フィールドが指すメソッドへ書き換える。
//
// HTMLのフォームがブラウザから送れるのはGETとPOSTだけのため、hiddenのフィールドを持たせることで
// PATCH / PUT / DELETEのルートをフォームから動かせるようにする。
// POST以外のリクエストと、未知の値を持つリクエストはそのまま通す。
//
// 書き換える先を状態を変えるメソッドに限るのは、POSTをGETやHEADへ落として、
// CSRFの検証を通さずにハンドラーへ届く経路を作らないため。
func MethodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		// ParseFormは解析した結果をリクエストに持たせるため、ここで読んだあとも
		// 後続のハンドラーは他のフォームの値をそのまま読める。
		if err := r.ParseForm(); err == nil {
			switch method := strings.ToUpper(r.PostFormValue(MethodOverrideFieldName)); method {
			case http.MethodPut, http.MethodPatch, http.MethodDelete:
				r.Method = method
			}
		}

		next.ServeHTTP(w, r)
	})
}
