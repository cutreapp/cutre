package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// GoodsRepository はgoods (グッズ) を読み書きする。
type GoodsRepository struct {
	q *query.Queries
}

// NewGoodsRepository は GoodsRepository を生成する。
func NewGoodsRepository(db *sql.DB) *GoodsRepository {
	return &GoodsRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい GoodsRepository を返す。
func (r *GoodsRepository) WithTx(tx *sql.Tx) *GoodsRepository {
	return &GoodsRepository{q: r.q.WithTx(tx)}
}

// FindByID は指定したIDのグッズを状態を問わずに返す。存在しない場合は (nil, nil) を返す。
// 削除したグッズや、削除したカテゴリー・イベントのグッズを存在しないものとして扱うかは呼び出し側が決める。
func (r *GoodsRepository) FindByID(ctx context.Context, id model.GoodsID) (*model.Goods, error) {
	row, err := r.q.GetGoodsByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// ListByIDs は指定したIDのグッズを、FindByID と同じく状態を問わずにまとめて返す。並び順は決めない。
// 一覧に出すアイテムのグッズを1回のクエリで引くのに使う。ids が空ならクエリを発行せずにnilを返す。
func (r *GoodsRepository) ListByIDs(ctx context.Context, ids []model.GoodsID) ([]*model.Goods, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	uuids := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = uuid.UUID(id)
	}
	rows, err := r.q.ListGoodsByIDs(ctx, uuids)
	if err != nil {
		return nil, err
	}

	goods := make([]*model.Goods, len(rows))
	for i, row := range rows {
		goods[i] = r.toModel(row)
	}

	return goods, nil
}

// LockByID はグッズの行をトランザクションの終了までロックする。
// 無いグッズでもエラーにしない。有無と状態は、呼び出し側がロックの取得後に別の文で読み直して判定する。
func (r *GoodsRepository) LockByID(ctx context.Context, id model.GoodsID) error {
	if _, err := r.q.LockGoodsByID(ctx, uuid.UUID(id)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// ListUndeletedByEventCategoryID は、カテゴリーの削除していない (公開中とアーカイブした) グッズを並び順に返す。
func (r *GoodsRepository) ListUndeletedByEventCategoryID(ctx context.Context, eventCategoryID model.EventCategoryID) ([]*model.Goods, error) {
	rows, err := r.q.ListUndeletedGoodsByEventCategoryID(ctx, uuid.UUID(eventCategoryID))
	if err != nil {
		return nil, err
	}

	goods := make([]*model.Goods, len(rows))
	for i, row := range rows {
		goods[i] = r.toModel(row)
	}

	return goods, nil
}

// ListPublishedByEventCategoryID は、カテゴリーの公開中のグッズを並び順に返す。カテゴリーとイベントが公開中かは呼び出し側が確かめる。
func (r *GoodsRepository) ListPublishedByEventCategoryID(ctx context.Context, eventCategoryID model.EventCategoryID) ([]*model.Goods, error) {
	rows, err := r.q.ListPublishedGoodsByEventCategoryID(ctx, uuid.UUID(eventCategoryID))
	if err != nil {
		return nil, err
	}

	goods := make([]*model.Goods, len(rows))
	for i, row := range rows {
		goods[i] = r.toModel(row)
	}

	return goods, nil
}

// CountPublishedGroupByEventID は、イベントごとの公開中のグッズの種類の数を返す。公開中のカテゴリーのグッズだけを数える。
// 公開中のグッズが無いイベントはmapに入らない。
func (r *GoodsRepository) CountPublishedGroupByEventID(ctx context.Context) (map[model.EventID]int64, error) {
	rows, err := r.q.CountPublishedGoodsGroupByEventID(ctx)
	if err != nil {
		return nil, err
	}

	counts := make(map[model.EventID]int64, len(rows))
	for _, row := range rows {
		counts[model.EventID(row.EventID)] = row.GoodsCount
	}

	return counts, nil
}

// CountPublishedByEventIDGroupByEventCategoryID は、イベントのカテゴリーごとの公開中のグッズの種類の数を返す。
// 公開中のグッズが無いカテゴリーはmapに入らない。
func (r *GoodsRepository) CountPublishedByEventIDGroupByEventCategoryID(ctx context.Context, eventID model.EventID) (map[model.EventCategoryID]int64, error) {
	rows, err := r.q.CountPublishedGoodsByEventIDGroupByEventCategoryID(ctx, uuid.UUID(eventID))
	if err != nil {
		return nil, err
	}

	counts := make(map[model.EventCategoryID]int64, len(rows))
	for _, row := range rows {
		counts[model.EventCategoryID(row.EventCategoryID)] = row.GoodsCount
	}

	return counts, nil
}

// GoodsAttributes は管理画面の編集のフォームで決めるグッズの属性。
type GoodsAttributes struct {
	Name     string
	Position int32
}

// Create はカテゴリー eventCategoryID の配下に公開中のグッズを挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *GoodsRepository) Create(ctx context.Context, eventCategoryID model.EventCategoryID, attrs GoodsAttributes) (*model.Goods, error) {
	row, err := r.q.CreateGoods(ctx, query.CreateGoodsParams{
		EventCategoryID: uuid.UUID(eventCategoryID),
		Name:            attrs.Name,
		Position:        attrs.Position,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// Update は、グッズの版が lockVersion と一致するときだけ属性を更新し、版を1つ上げる。
// 版が一致しない (ほかの操作で先に更新された) か、削除していて更新しなかったときはfalseを返す。
func (r *GoodsRepository) Update(ctx context.Context, id model.GoodsID, lockVersion int32, attrs GoodsAttributes) (bool, error) {
	affected, err := r.q.UpdateGoods(ctx, query.UpdateGoodsParams{
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

// Archive は公開中のグッズを、理由 archiveMessage を残してアーカイブする。
// 公開中でないか版が一致せず更新しなかったときはfalseを返す。
func (r *GoodsRepository) Archive(ctx context.Context, id model.GoodsID, lockVersion int32, archiveMessage string) (bool, error) {
	affected, err := r.q.ArchiveGoods(ctx, query.ArchiveGoodsParams{
		ID:             uuid.UUID(id),
		ArchiveMessage: &archiveMessage,
		LockVersion:    lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Unarchive はアーカイブしたグッズを公開に戻し、理由を空にする。
// アーカイブしていないか版が一致せず更新しなかったときはfalseを返す。
func (r *GoodsRepository) Unarchive(ctx context.Context, id model.GoodsID, lockVersion int32) (bool, error) {
	affected, err := r.q.UnarchiveGoods(ctx, query.UnarchiveGoodsParams{ID: uuid.UUID(id), LockVersion: lockVersion})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Delete はグッズを削除した状態にする。行は消さない。
// 既に削除しているか版が一致せず更新しなかったときはfalseを返す。
func (r *GoodsRepository) Delete(ctx context.Context, id model.GoodsID, lockVersion int32) (bool, error) {
	affected, err := r.q.DeleteGoods(ctx, query.DeleteGoodsParams{ID: uuid.UUID(id), LockVersion: lockVersion})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// toModel はクエリの行を model.Goods に変換する。
func (r *GoodsRepository) toModel(row query.Goods) *model.Goods {
	return &model.Goods{
		ID:              model.GoodsID(row.ID),
		EventCategoryID: model.EventCategoryID(row.EventCategoryID),
		Name:            row.Name,
		Position:        row.Position,
		Status:          model.MasterStatus(row.Status),
		ArchiveMessage:  row.ArchiveMessage,
		LockVersion:     row.LockVersion,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
