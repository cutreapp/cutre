package model

import "time"

// EventCategory はイベントのカテゴリー (例: 「B賞 ラバーマスコット」) のマスタ。イベントの配下にあり、配下にグッズを持つ。
//
// Position はイベントの中での並び順で、小さいものから並べる。
// ArchiveMessage と LockVersion の意味はイベント (Event) と同じ。
type EventCategory struct {
	ID             EventCategoryID
	EventID        EventID
	Name           string
	Position       int32
	Status         MasterStatus
	ArchiveMessage *string
	LockVersion    int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsPublished は公開中かを返す。ユーザー向けの一覧と選択肢には、自分と上位のマスタがすべて公開中のものだけを出す。
func (c *EventCategory) IsPublished() bool {
	return c.Status == MasterStatusPublished
}

// IsArchived はアーカイブしているかを返す。
func (c *EventCategory) IsArchived() bool {
	return c.Status == MasterStatusArchived
}

// IsDeleted は削除したかを返す。削除したカテゴリーは管理画面でも存在しないものとして扱う。
func (c *EventCategory) IsDeleted() bool {
	return c.Status == MasterStatusDeleted
}
