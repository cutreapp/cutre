package viewmodel

import (
	"strconv"

	"github.com/cutreapp/cutre/go/internal/model"
)

// AdminMasterRow は管理画面のカテゴリー・グッズの一覧の1行。
type AdminMasterRow struct {
	ID       string
	Name     string
	Archived bool
}

// NewAdminEventCategoryRows はカテゴリーを管理画面の一覧の行にする。
func NewAdminEventCategoryRows(categories []*model.EventCategory) []AdminMasterRow {
	rows := make([]AdminMasterRow, len(categories))
	for i, category := range categories {
		rows[i] = AdminMasterRow{ID: category.ID.String(), Name: category.Name, Archived: category.IsArchived()}
	}

	return rows
}

// NewAdminGoodsRows はグッズを管理画面の一覧の行にする。
func NewAdminGoodsRows(goods []*model.Goods) []AdminMasterRow {
	rows := make([]AdminMasterRow, len(goods))
	for i, g := range goods {
		rows[i] = AdminMasterRow{ID: g.ID.String(), Name: g.Name, Archived: g.IsArchived()}
	}

	return rows
}

// MasterForm は管理画面のカテゴリー・グッズの作成・編集のフォームに入れる値。
// 並び順は入力欄の値の文字列で持ち、エラーで描き直すときは送られた値をそのまま戻す。
type MasterForm struct {
	Name     string
	Position string
	// LockVersion は編集のフォームが持ち回る版。作成のフォームでは使わない。
	LockVersion int32
}

// NewEventCategoryForm は保存済みのカテゴリーを、編集のフォームに入れる値にする。
func NewEventCategoryForm(category *model.EventCategory) MasterForm {
	return MasterForm{Name: category.Name, Position: formatPosition(category.Position), LockVersion: category.LockVersion}
}

// NewGoodsForm は保存済みのグッズを、編集のフォームに入れる値にする。
func NewGoodsForm(goods *model.Goods) MasterForm {
	return MasterForm{Name: goods.Name, Position: formatPosition(goods.Position), LockVersion: goods.LockVersion}
}

// NewEventCategoryCreateForm はカテゴリーの作成のフォームの初期値を返す。
// 並び順には、イベントの既存のカテゴリー (並び順に並んだもの) の最後の値に100を足した値を入れ、カテゴリーが無ければ100を入れる。
// 上限の9999を超えるときは9999を使い、同じ並び順のカテゴリーはID順に並べる。
func NewEventCategoryCreateForm(categories []*model.EventCategory) MasterForm {
	if len(categories) == 0 {
		return MasterForm{Position: formatPosition(masterPositionStep)}
	}

	return MasterForm{Position: nextMasterPosition(categories[len(categories)-1].Position)}
}

// NewGoodsCreateForm はグッズの作成のフォームの初期値を返す。
// 並び順には、カテゴリーの既存のグッズ (並び順に並んだもの) の最後の値に100を足した値を入れ、グッズが無ければ100を入れる。
// 上限の9999を超えるときは9999を使い、同じ並び順のグッズはID順に並べる。
func NewGoodsCreateForm(goods []*model.Goods) MasterForm {
	if len(goods) == 0 {
		return MasterForm{Position: formatPosition(masterPositionStep)}
	}

	return MasterForm{Position: nextMasterPosition(goods[len(goods)-1].Position)}
}

// masterPositionStep は作成のフォームに入れる並び順の間隔。
// 並べ替えは数値の入力で行うため、あとから間に差し込めるよう1つずつではなく間を空ける。
const masterPositionStep = 100

// nextMasterPosition は最後の並び順 last の次の並び順を、フォームで受け付ける範囲の値として返す。
func nextMasterPosition(last int32) string {
	const maxPosition = 9999
	if last > maxPosition-masterPositionStep {
		return formatPosition(maxPosition)
	}

	return formatPosition(last + masterPositionStep)
}

// formatPosition は並び順を入力欄の値にする。
func formatPosition(position int32) string {
	return strconv.FormatInt(int64(position), 10)
}
