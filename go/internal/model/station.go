package model

import "time"

// Station は交換場所に選べる主要駅のマスタ。都道府県ごとに運営が用意する。
//
// Position は都道府県の中での並び順で、小さいものから並べる。
// ArchiveMessage と LockVersion の意味はイベント (Event) と同じ。
type Station struct {
	ID             StationID
	PrefectureCode PrefectureCode
	Name           string
	Position       int32
	Status         MasterStatus
	ArchiveMessage *string
	LockVersion    int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsArchived はアーカイブしているかを返す。
func (s *Station) IsArchived() bool {
	return s.Status == MasterStatusArchived
}

// IsDeleted は削除したかを返す。削除した駅は管理画面でも存在しないものとして扱う。
func (s *Station) IsDeleted() bool {
	return s.Status == MasterStatusDeleted
}
