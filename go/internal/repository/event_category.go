package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// EventCategoryRepository はevent_categories (カテゴリー) を読み書きする。
type EventCategoryRepository struct {
	q *query.Queries
}

// NewEventCategoryRepository は EventCategoryRepository を生成する。
func NewEventCategoryRepository(db *sql.DB) *EventCategoryRepository {
	return &EventCategoryRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい EventCategoryRepository を返す。
func (r *EventCategoryRepository) WithTx(tx *sql.Tx) *EventCategoryRepository {
	return &EventCategoryRepository{q: r.q.WithTx(tx)}
}

// FindByID は指定したIDのカテゴリーを状態を問わずに返す。存在しない場合は (nil, nil) を返す。
// 削除したカテゴリーや、削除したイベントのカテゴリーを存在しないものとして扱うかは呼び出し側が決める。
func (r *EventCategoryRepository) FindByID(ctx context.Context, id model.EventCategoryID) (*model.EventCategory, error) {
	row, err := r.q.GetEventCategoryByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// ListByIDs は指定したIDのカテゴリーを、FindByID と同じく状態を問わずにまとめて返す。並び順は決めない。
// 一覧に出すアイテムのカテゴリーを1回のクエリで引くのに使う。ids が空ならクエリを発行せずにnilを返す。
func (r *EventCategoryRepository) ListByIDs(ctx context.Context, ids []model.EventCategoryID) ([]*model.EventCategory, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	uuids := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = uuid.UUID(id)
	}
	rows, err := r.q.ListEventCategoriesByIDs(ctx, uuids)
	if err != nil {
		return nil, err
	}

	categories := make([]*model.EventCategory, len(rows))
	for i, row := range rows {
		categories[i] = r.toModel(row)
	}

	return categories, nil
}

// LockByID はカテゴリーの行をトランザクションの終了までロックする。
// 無いカテゴリーでもエラーにしない。有無と状態は、呼び出し側がロックの取得後に別の文で読み直して判定する。
func (r *EventCategoryRepository) LockByID(ctx context.Context, id model.EventCategoryID) error {
	if _, err := r.q.LockEventCategoryByID(ctx, uuid.UUID(id)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// ListUndeletedByEventID は、イベントの削除していない (公開中とアーカイブした) カテゴリーを並び順に返す。
func (r *EventCategoryRepository) ListUndeletedByEventID(ctx context.Context, eventID model.EventID) ([]*model.EventCategory, error) {
	rows, err := r.q.ListUndeletedEventCategoriesByEventID(ctx, uuid.UUID(eventID))
	if err != nil {
		return nil, err
	}

	categories := make([]*model.EventCategory, len(rows))
	for i, row := range rows {
		categories[i] = r.toModel(row)
	}

	return categories, nil
}

// ListPublishedByEventID は、イベントの公開中のカテゴリーを並び順に返す。イベントが公開中かは呼び出し側が確かめる。
func (r *EventCategoryRepository) ListPublishedByEventID(ctx context.Context, eventID model.EventID) ([]*model.EventCategory, error) {
	rows, err := r.q.ListPublishedEventCategoriesByEventID(ctx, uuid.UUID(eventID))
	if err != nil {
		return nil, err
	}

	categories := make([]*model.EventCategory, len(rows))
	for i, row := range rows {
		categories[i] = r.toModel(row)
	}

	return categories, nil
}

// EventCategoryAttributes は管理画面の編集のフォームで決めるカテゴリーの属性。
type EventCategoryAttributes struct {
	Name     string
	Position int32
}

// Create はイベント eventID の配下に公開中のカテゴリーを挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *EventCategoryRepository) Create(ctx context.Context, eventID model.EventID, attrs EventCategoryAttributes) (*model.EventCategory, error) {
	row, err := r.q.CreateEventCategory(ctx, query.CreateEventCategoryParams{
		EventID:  uuid.UUID(eventID),
		Name:     attrs.Name,
		Position: attrs.Position,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// Update は、カテゴリーの版が lockVersion と一致するときだけ属性を更新し、版を1つ上げる。
// 版が一致しない (ほかの操作で先に更新された) か、削除していて更新しなかったときはfalseを返す。
func (r *EventCategoryRepository) Update(ctx context.Context, id model.EventCategoryID, lockVersion int32, attrs EventCategoryAttributes) (bool, error) {
	affected, err := r.q.UpdateEventCategory(ctx, query.UpdateEventCategoryParams{
		ID:          uuid.UUID(id),
		Name:        attrs.Name,
		Position:    attrs.Position,
		LockVersion: lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Archive は公開中のカテゴリーを、理由 archiveMessage を残してアーカイブする。
// 公開中でないか版が一致せず更新しなかったときはfalseを返す。
func (r *EventCategoryRepository) Archive(ctx context.Context, id model.EventCategoryID, lockVersion int32, archiveMessage string) (bool, error) {
	affected, err := r.q.ArchiveEventCategory(ctx, query.ArchiveEventCategoryParams{
		ID:             uuid.UUID(id),
		ArchiveMessage: &archiveMessage,
		LockVersion:    lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Unarchive はアーカイブしたカテゴリーを公開に戻し、理由を空にする。
// アーカイブしていないか版が一致せず更新しなかったときはfalseを返す。
func (r *EventCategoryRepository) Unarchive(ctx context.Context, id model.EventCategoryID, lockVersion int32) (bool, error) {
	affected, err := r.q.UnarchiveEventCategory(ctx, query.UnarchiveEventCategoryParams{ID: uuid.UUID(id), LockVersion: lockVersion})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Delete はカテゴリーを削除した状態にする。行は消さない。
// 既に削除しているか版が一致せず更新しなかったときはfalseを返す。
func (r *EventCategoryRepository) Delete(ctx context.Context, id model.EventCategoryID, lockVersion int32) (bool, error) {
	affected, err := r.q.DeleteEventCategory(ctx, query.DeleteEventCategoryParams{ID: uuid.UUID(id), LockVersion: lockVersion})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// toModel はクエリの行を model.EventCategory に変換する。
func (r *EventCategoryRepository) toModel(row query.EventCategory) *model.EventCategory {
	return &model.EventCategory{
		ID:             model.EventCategoryID(row.ID),
		EventID:        model.EventID(row.EventID),
		Name:           row.Name,
		Position:       row.Position,
		Status:         model.MasterStatus(row.Status),
		ArchiveMessage: row.ArchiveMessage,
		LockVersion:    row.LockVersion,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
