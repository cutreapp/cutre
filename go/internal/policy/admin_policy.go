// Package policy はユーザーが操作をしてよいかの判定 (認可) を提供する。
// 判定はUseCaseから呼び、ハンドラーは呼ばない。
package policy

import "github.com/cutreapp/cutre/go/internal/model"

// AdminPolicy は管理画面の操作をしてよいかを、操作するユーザーの役割で判定する。
type AdminPolicy struct {
	user *model.User
}

// NewAdminPolicy は user が操作するときの AdminPolicy を生成する。
// user がnil (ログインしていない) のときは、どの操作も許さない。
func NewAdminPolicy(user *model.User) *AdminPolicy {
	return &AdminPolicy{user: user}
}

// CanUseAdmin は管理画面を開き、マスタを作成・編集・アーカイブ・元に戻すことができるかを返す。
// 編集者と管理者に許す。
func (p *AdminPolicy) CanUseAdmin() bool {
	return p.user != nil && p.user.IsEditorOrAbove()
}

// CanDeleteMaster はマスタを削除できるかを返す。
// 削除は元に戻せないため、管理者だけに許す。編集者は代わりにアーカイブする。
func (p *AdminPolicy) CanDeleteMaster() bool {
	return p.user != nil && p.user.IsAdmin()
}
