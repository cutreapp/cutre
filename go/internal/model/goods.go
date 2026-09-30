package model

import "time"

// Goods はカテゴリーの中のグッズ (例: 「くまの子」) のマスタ。ユーザーはグッズを譲れる・ほしいのリストに入れる。
//
// 「goods」は単数と複数が同じ形のため、1つのグッズも Goods と呼ぶ。
// Position はカテゴリーの中での並び順で、小さいものから並べる。
// ArchiveMessage と LockVersion の意味はイベント (Event) と同じ。
type Goods struct {
	ID              GoodsID
	EventCategoryID EventCategoryID
	Name            string
	Position        int32
	Status          MasterStatus
	ArchiveMessage  *string
	LockVersion     int32
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsPublished は公開中かを返す。ユーザー向けの一覧と選択肢には、自分と上位のマスタがすべて公開中のものだけを出す。
func (g *Goods) IsPublished() bool {
	return g.Status == MasterStatusPublished
}

// IsArchived はアーカイブしているかを返す。
func (g *Goods) IsArchived() bool {
	return g.Status == MasterStatusArchived
}

// IsDeleted は削除したかを返す。削除したグッズは管理画面でも存在しないものとして扱う。
func (g *Goods) IsDeleted() bool {
	return g.Status == MasterStatusDeleted
}
