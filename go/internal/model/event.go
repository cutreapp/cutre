package model

import "time"

// MasterStatus は運営が管理画面で用意するマスタ (イベント・カテゴリー・グッズ・駅) の状態。
//
// データベースではテーブルごとに別のENUM型 (event_status など) で持つが、値はどれも同じのため、
// Goではこの1つの型で扱い、状態を見る処理をマスタの種類ごとに書き分けずに済むようにする。
type MasterStatus string

const (
	// MasterStatusPublished は公開中。ユーザー向けの一覧と選択肢に出る。
	MasterStatusPublished MasterStatus = "published"
	// MasterStatusArchived はアーカイブした状態。ユーザー向けの一覧と選択肢には出ないが、
	// すでにリストや交換にあるものはそのまま使え、管理画面から公開に戻せる。
	MasterStatusArchived MasterStatus = "archived"
	// MasterStatusDeleted は削除した状態。管理画面にも出さず、あとから物理削除する。
	MasterStatusDeleted MasterStatus = "deleted"
)

// Event は運営が用意するイベント (例: 一番くじ) のマスタ。配下にカテゴリーを持つ。
//
// 開催期間は暦日で持つ。StartsOn と EndsOn は時刻を持たないUTCの0時で、
// タイムゾーンを変換せずに日付として読む。EndsOn がnilのイベントは終わりが決まっていない。
//
// ArchiveMessage はアーカイブした理由で、管理画面にだけ出す。アーカイブしていないときはnil。
// LockVersion は管理画面の編集の競合を見つけるための版で、更新のたびに1つ上がる。列の型に合わせてint32で持つ。
type Event struct {
	ID             EventID
	Name           string
	StartsOn       time.Time
	EndsOn         *time.Time
	Status         MasterStatus
	ArchiveMessage *string
	LockVersion    int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsPublished は公開中かを返す。ユーザー向けの一覧と選択肢には、自分と上位のマスタがすべて公開中のものだけを出す。
func (e *Event) IsPublished() bool {
	return e.Status == MasterStatusPublished
}

// IsArchived はアーカイブしているかを返す。
func (e *Event) IsArchived() bool {
	return e.Status == MasterStatusArchived
}

// IsDeleted は削除したかを返す。削除したイベントは管理画面でも存在しないものとして扱う。
func (e *Event) IsDeleted() bool {
	return e.Status == MasterStatusDeleted
}
