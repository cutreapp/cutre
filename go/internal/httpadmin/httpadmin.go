// Package httpadmin は管理画面のハンドラーが共有する、リクエストの読み取りとエラーの応え方の判定を提供する。
//
// マスタ (イベント・カテゴリー・グッズ・駅) の管理のハンドラーは、どれもURLのIDを読み、
// 管理画面を使えないユーザーに管理画面の存在を明かさない。その決まりをマスタの種類ごとに書き分けず、ここで1つに持つ。
package httpadmin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// ParseIDParam はURLのパラメータ name をUUIDとして読み、ID の型にして返す。
// UUIDとして読めないときはfalseを返す。呼び出し元は存在しないページ (404) として扱う。
func ParseIDParam[ID ~[16]byte](r *http.Request, name string) (ID, bool) {
	parsed, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		var zero ID
		return zero, false
	}

	return ID(parsed), true
}

// IsNotFound は、UseCaseのエラーが存在しないページとして404で応えるものかを返す。
// 管理画面を使えない (AppErrCodeForbidden) ときも、対象が無い (AppErrCodeResourceNotFound) ときも404にし、
// 一般のユーザーに管理画面の存在を明かさない。
func IsNotFound(err error) bool {
	ae := model.AsAppError(err)

	return ae != nil && (ae.Code == model.AppErrCodeForbidden || ae.Code == model.AppErrCodeResourceNotFound)
}

// DeleteFailure は、マスタの削除のUseCaseのエラーのうち、編集の画面を描き直して伝えるものを、
// 応えるステータスと編集の画面に出すエラーにして返す。
//
// 参照があって削除できないとき (*model.ValidationError) は422とそのエラーを、
// 画面を開いたあとにほかの操作で更新されていたとき (AppErrCodeConflict) は409と conflictMessage のエラーを返す。
// それ以外のエラーには (0, nil) を返し、呼び出し元がほかの応え方を決める。
func DeleteFailure(err error, conflictMessage string) (int, *model.ValidationError) {
	if ve := model.AsValidationError(err); ve != nil {
		return http.StatusUnprocessableEntity, ve
	}
	if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
		conflict := model.NewValidationError()
		conflict.AddGlobal(conflictMessage)
		return http.StatusConflict, conflict
	}

	return 0, nil
}
