package usecase

import (
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
)

// authorizeAdmin は、user が管理画面を使える (編集者か管理者) かを確かめる。
// 使えないときは AppErrCodeForbidden の *model.AppError を返す。
// ハンドラーはこれを存在しないページ (404) として扱い、一般のユーザーに管理画面の存在を明かさない。
func authorizeAdmin(user *model.User) error {
	if policy.NewAdminPolicy(user).CanUseAdmin() {
		return nil
	}

	return forbiddenAdminError(user)
}

// authorizeMasterDeletion は、user がマスタを削除できる (管理者) かを確かめる。
// できないときは AppErrCodeForbidden の *model.AppError を返す。
func authorizeMasterDeletion(user *model.User) error {
	if policy.NewAdminPolicy(user).CanDeleteMaster() {
		return nil
	}

	return forbiddenAdminError(user)
}

// forbiddenAdminError は管理画面の操作を許さないときのエラーを返す。
func forbiddenAdminError(user *model.User) error {
	ae := &model.AppError{Code: model.AppErrCodeForbidden, Internal: fmt.Errorf("管理画面の操作の権限がありません")}
	if user != nil {
		ae.Metadata = map[string]string{"user_id": user.ID.String(), "role": string(user.Role)}
	}

	return ae
}
