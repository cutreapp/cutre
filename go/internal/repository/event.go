package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// EventRepository はeventsを読み書きする。
type EventRepository struct {
	q *query.Queries
}

// NewEventRepository は EventRepository を生成する。
func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい EventRepository を返す。
func (r *EventRepository) WithTx(tx *sql.Tx) *EventRepository {
	return &EventRepository{q: r.q.WithTx(tx)}
}

// FindByID は指定したIDのイベントを状態を問わずに返す。存在しない場合は (nil, nil) を返す。
// 削除したイベントを存在しないものとして扱うかは呼び出し側が決める。
func (r *EventRepository) FindByID(ctx context.Context, id model.EventID) (*model.Event, error) {
	row, err := r.q.GetEventByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// ListByIDs は指定したIDのイベントを、FindByID と同じく状態を問わずにまとめて返す。並び順は決めない。
// 一覧に出すアイテムのイベントを1回のクエリで引くのに使う。ids が空ならクエリを発行せずにnilを返す。
func (r *EventRepository) ListByIDs(ctx context.Context, ids []model.EventID) ([]*model.Event, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	uuids := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = uuid.UUID(id)
	}
	rows, err := r.q.ListEventsByIDs(ctx, uuids)
	if err != nil {
		return nil, err
	}

	events := make([]*model.Event, len(rows))
	for i, row := range rows {
		events[i] = r.toModel(row)
	}

	return events, nil
}

// LockByID はイベントの行をトランザクションの終了までロックする。
// 無いイベントでもエラーにしない。有無と状態は、呼び出し側がロックの取得後に別の文で読み直して判定する。
func (r *EventRepository) LockByID(ctx context.Context, id model.EventID) error {
	if _, err := r.q.LockEventByID(ctx, uuid.UUID(id)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// ListUndeleted は削除していない (公開中とアーカイブした) イベントを、開始日の新しい順に返す。
func (r *EventRepository) ListUndeleted(ctx context.Context) ([]*model.Event, error) {
	rows, err := r.q.ListUndeletedEvents(ctx)
	if err != nil {
		return nil, err
	}

	events := make([]*model.Event, len(rows))
	for i, row := range rows {
		events[i] = r.toModel(row)
	}

	return events, nil
}

// ListPublished は公開中のイベントを、開始日の新しい順に返す。ユーザー向けの一覧に使う。
func (r *EventRepository) ListPublished(ctx context.Context) ([]*model.Event, error) {
	rows, err := r.q.ListPublishedEvents(ctx)
	if err != nil {
		return nil, err
	}

	events := make([]*model.Event, len(rows))
	for i, row := range rows {
		events[i] = r.toModel(row)
	}

	return events, nil
}

// EventAttributes は管理画面の編集のフォームで決めるイベントの属性。
type EventAttributes struct {
	Name     string
	StartsOn time.Time
	// EndsOn は終わりの日。nilは終わりが決まっていないことを表す。
	EndsOn *time.Time
}

// Create は公開中のイベントを挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *EventRepository) Create(ctx context.Context, attrs EventAttributes) (*model.Event, error) {
	row, err := r.q.CreateEvent(ctx, query.CreateEventParams{
		Name:     attrs.Name,
		StartsOn: attrs.StartsOn,
		EndsOn:   attrs.EndsOn,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// Update は、イベントの版が lockVersion と一致するときだけ属性を更新し、版を1つ上げる。
// 版が一致しない (ほかの操作で先に更新された) か、削除していて更新しなかったときはfalseを返す。
func (r *EventRepository) Update(ctx context.Context, id model.EventID, lockVersion int32, attrs EventAttributes) (bool, error) {
	affected, err := r.q.UpdateEvent(ctx, query.UpdateEventParams{
		ID:          uuid.UUID(id),
		Name:        attrs.Name,
		StartsOn:    attrs.StartsOn,
		EndsOn:      attrs.EndsOn,
		LockVersion: lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Archive は公開中のイベントを、理由 archiveMessage を残してアーカイブする。
// 公開中でないか版が一致せず更新しなかったときはfalseを返す。
func (r *EventRepository) Archive(ctx context.Context, id model.EventID, lockVersion int32, archiveMessage string) (bool, error) {
	affected, err := r.q.ArchiveEvent(ctx, query.ArchiveEventParams{
		ID:             uuid.UUID(id),
		ArchiveMessage: &archiveMessage,
		LockVersion:    lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Unarchive はアーカイブしたイベントを公開に戻し、理由を空にする。
// アーカイブしていないか版が一致せず更新しなかったときはfalseを返す。
func (r *EventRepository) Unarchive(ctx context.Context, id model.EventID, lockVersion int32) (bool, error) {
	affected, err := r.q.UnarchiveEvent(ctx, query.UnarchiveEventParams{ID: uuid.UUID(id), LockVersion: lockVersion})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Delete はイベントを削除した状態にする。行は消さない。
// 既に削除しているか版が一致せず更新しなかったときはfalseを返す。
func (r *EventRepository) Delete(ctx context.Context, id model.EventID, lockVersion int32) (bool, error) {
	affected, err := r.q.DeleteEvent(ctx, query.DeleteEventParams{ID: uuid.UUID(id), LockVersion: lockVersion})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// toModel はクエリの行を model.Event に変換する。
func (r *EventRepository) toModel(row query.Event) *model.Event {
	return &model.Event{
		ID:             model.EventID(row.ID),
		Name:           row.Name,
		StartsOn:       row.StartsOn,
		EndsOn:         row.EndsOn,
		Status:         model.MasterStatus(row.Status),
		ArchiveMessage: row.ArchiveMessage,
		LockVersion:    row.LockVersion,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
