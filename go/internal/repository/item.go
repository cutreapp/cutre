package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// ItemRepository はitems (リストのアイテム) を読み書きする。
type ItemRepository struct {
	q *query.Queries
}

// NewItemRepository は ItemRepository を生成する。
func NewItemRepository(db *sql.DB) *ItemRepository {
	return &ItemRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい ItemRepository を返す。
func (r *ItemRepository) WithTx(tx *sql.Tx) *ItemRepository {
	return &ItemRepository{q: r.q.WithTx(tx)}
}

// ItemAttributes はリストに追加するフォームで決めるアイテムの属性。
type ItemAttributes struct {
	Kind     model.ItemKind
	Quantity int32
	Note     string
}

// Create はユーザー userID のリストに、グッズ goodsID のアイテムを入れ、データベースが採番したidとタイムスタンプを含めて返す。
// 同じリストに同じグッズのアイテムが既にあるときは ErrItemAlreadyListed を返す。
func (r *ItemRepository) Create(ctx context.Context, userID model.UserID, goodsID model.GoodsID, attrs ItemAttributes) (*model.Item, error) {
	row, err := r.q.CreateItem(ctx, query.CreateItemParams{
		UserID:   uuid.UUID(userID),
		GoodsID:  uuid.UUID(goodsID),
		Kind:     query.ItemKind(attrs.Kind),
		Quantity: attrs.Quantity,
		Note:     attrs.Note,
	})
	if err != nil {
		if uniqueViolationConstraint(err) == "items_user_id_goods_id_kind_idx" {
			return nil, ErrItemAlreadyListed
		}
		return nil, err
	}

	return r.toModel(row), nil
}

// FindByID は指定したIDのアイテムを、状態と持ち主を問わずに返す。存在しない場合は (nil, nil) を返す。
// 外したアイテムや、ほかのユーザーのアイテムを存在しないものとして扱うかは呼び出し側が決める。
func (r *ItemRepository) FindByID(ctx context.Context, id model.ItemID) (*model.Item, error) {
	row, err := r.q.GetItemByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// ListListedByUserIDAndKind は、ユーザー userID のリスト kind にあるアイテムを返す。
// イベントを開始日の新しい順に、イベントの中はカテゴリー・グッズの並び順に並べる。マスタの状態は問わない。
func (r *ItemRepository) ListListedByUserIDAndKind(ctx context.Context, userID model.UserID, kind model.ItemKind) ([]*model.Item, error) {
	rows, err := r.q.ListListedItemsByUserIDAndKind(ctx, query.ListListedItemsByUserIDAndKindParams{
		UserID: uuid.UUID(userID),
		Kind:   query.ItemKind(kind),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*model.Item, len(rows))
	for i, row := range rows {
		items[i] = r.toModel(row)
	}

	return items, nil
}

// SumListedQuantitiesByUserID は、ユーザー userID のリストにあるアイテムの数量を、リストごとに合計して返す。
func (r *ItemRepository) SumListedQuantitiesByUserID(ctx context.Context, userID model.UserID) (model.ItemQuantities, error) {
	rows, err := r.q.SumListedItemQuantitiesByUserIDGroupByKind(ctx, uuid.UUID(userID))
	if err != nil {
		return model.ItemQuantities{}, err
	}

	var sum model.ItemQuantities
	for _, row := range rows {
		sum.Add(model.ItemKind(row.Kind), row.Quantity)
	}

	return sum, nil
}

// ItemUpdateAttributes はリストのアイテムの編集のフォームで決めるアイテムの属性。
// 入れたリストは変えられないため持たない。
type ItemUpdateAttributes struct {
	Quantity int32
	Note     string
}

// UpdateListed は、ユーザー userID のリストにあるアイテム id の数量とひとことを更新する。
// 版が lockVersion と一致するときだけ更新して版を進める。更新しなかったときはfalseを返す。
func (r *ItemRepository) UpdateListed(ctx context.Context, id model.ItemID, userID model.UserID, lockVersion int32, attrs ItemUpdateAttributes) (bool, error) {
	affected, err := r.q.UpdateListedItem(ctx, query.UpdateListedItemParams{
		ID:          uuid.UUID(id),
		UserID:      uuid.UUID(userID),
		Quantity:    attrs.Quantity,
		Note:        attrs.Note,
		LockVersion: lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// RemoveListed は、ユーザー userID のリストにあるアイテム id をリストから外す (状態を removed にする)。
// 版が lockVersion と一致するときだけ外して版を進める。外さなかったときはfalseを返す。
func (r *ItemRepository) RemoveListed(ctx context.Context, id model.ItemID, userID model.UserID, lockVersion int32) (bool, error) {
	affected, err := r.q.RemoveListedItem(ctx, query.RemoveListedItemParams{
		ID:          uuid.UUID(id),
		UserID:      uuid.UUID(userID),
		LockVersion: lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// RemoveListedByUserID は、ユーザー userID のリストにあるアイテムを、すべてリストから外す。退会で使う。
// 行は消さず、交換の品から辿れるよう残す。
func (r *ItemRepository) RemoveListedByUserID(ctx context.Context, userID model.UserID) error {
	return r.q.RemoveListedItemsByUserID(ctx, uuid.UUID(userID))
}

// ListListedByUserIDAndEventCategoryID は、カテゴリー eventCategoryID のグッズを指す、ユーザー userID のリストにあるアイテムを返す。
func (r *ItemRepository) ListListedByUserIDAndEventCategoryID(ctx context.Context, userID model.UserID, eventCategoryID model.EventCategoryID) ([]*model.Item, error) {
	rows, err := r.q.ListListedItemsByUserIDAndEventCategoryID(ctx, query.ListListedItemsByUserIDAndEventCategoryIDParams{
		UserID:          uuid.UUID(userID),
		EventCategoryID: uuid.UUID(eventCategoryID),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*model.Item, len(rows))
	for i, row := range rows {
		items[i] = r.toModel(row)
	}

	return items, nil
}

// SumListedQuantitiesByUserIDGroupByEventID は、ユーザー userID のリストにあるアイテムの数量を、イベントごと・リストごとに合計して返す。
// アイテムが無いイベントはmapに入らない。
func (r *ItemRepository) SumListedQuantitiesByUserIDGroupByEventID(ctx context.Context, userID model.UserID) (map[model.EventID]model.ItemQuantities, error) {
	rows, err := r.q.SumListedItemQuantitiesByUserIDGroupByEventID(ctx, uuid.UUID(userID))
	if err != nil {
		return nil, err
	}

	sums := make(map[model.EventID]model.ItemQuantities, len(rows))
	for _, row := range rows {
		id := model.EventID(row.EventID)
		sum := sums[id]
		sum.Add(model.ItemKind(row.Kind), row.Quantity)
		sums[id] = sum
	}

	return sums, nil
}

// SumListedQuantitiesByUserIDAndEventIDGroupByEventCategoryID は、イベント eventID の中で、
// ユーザー userID のリストにあるアイテムの数量を、カテゴリーごと・リストごとに合計して返す。アイテムが無いカテゴリーはmapに入らない。
func (r *ItemRepository) SumListedQuantitiesByUserIDAndEventIDGroupByEventCategoryID(ctx context.Context, userID model.UserID, eventID model.EventID) (map[model.EventCategoryID]model.ItemQuantities, error) {
	rows, err := r.q.SumListedItemQuantitiesByUserIDAndEventIDGroupByEventCategoryID(ctx, query.SumListedItemQuantitiesByUserIDAndEventIDGroupByEventCategoryIDParams{
		UserID:  uuid.UUID(userID),
		EventID: uuid.UUID(eventID),
	})
	if err != nil {
		return nil, err
	}

	sums := make(map[model.EventCategoryID]model.ItemQuantities, len(rows))
	for _, row := range rows {
		id := model.EventCategoryID(row.EventCategoryID)
		sum := sums[id]
		sum.Add(model.ItemKind(row.Kind), row.Quantity)
		sums[id] = sum
	}

	return sums, nil
}

// ExistsByGoodsID は、グッズ goodsID を参照するアイテムがあるかを返す。
// 外したアイテムも、交換の記録からグッズを辿るため数える。
func (r *ItemRepository) ExistsByGoodsID(ctx context.Context, goodsID model.GoodsID) (bool, error) {
	return r.q.ExistsItemByGoodsID(ctx, uuid.UUID(goodsID))
}

// ExistsByEventCategoryID は、カテゴリー eventCategoryID の配下のグッズ (状態を問わない) を参照するアイテムがあるかを返す。
// 外したアイテムも数える。
func (r *ItemRepository) ExistsByEventCategoryID(ctx context.Context, eventCategoryID model.EventCategoryID) (bool, error) {
	return r.q.ExistsItemByEventCategoryID(ctx, uuid.UUID(eventCategoryID))
}

// ExistsByEventID は、イベント eventID の配下のカテゴリー・グッズ (状態を問わない) を参照するアイテムがあるかを返す。
// 外したアイテムも数える。
func (r *ItemRepository) ExistsByEventID(ctx context.Context, eventID model.EventID) (bool, error) {
	return r.q.ExistsItemByEventID(ctx, uuid.UUID(eventID))
}

// ListListedMatching は、ユーザー userIDs のリストにあるアイテムのうち、
// 相手 counterpartUserIDs のだれかが同じグッズを反対のリスト (譲れるに対してほしい、ほしいに対して譲れる) に入れているものを返す。
// イベントを開始日の新しい順に、イベントの中はカテゴリー・グッズの並び順に並べる。マスタの状態は問わない。
// どちらかが空ならクエリを発行せずにnilを返す。
func (r *ItemRepository) ListListedMatching(ctx context.Context, userIDs, counterpartUserIDs []model.UserID) ([]*model.Item, error) {
	if len(userIDs) == 0 || len(counterpartUserIDs) == 0 {
		return nil, nil
	}

	rows, err := r.q.ListListedItemsMatchingUsers(ctx, query.ListListedItemsMatchingUsersParams{
		UserIds:            userUUIDs(userIDs),
		CounterpartUserIds: userUUIDs(counterpartUserIDs),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*model.Item, len(rows))
	for i, row := range rows {
		items[i] = r.toModel(row)
	}

	return items, nil
}

// LockByIDs は指定したIDのアイテムを、状態と持ち主を問わずにID順にロックして返す。トランザクション内で使う。
// 交換の申し込みでは、状態の確認から作成までリストから外す操作を待たせる。ids が空ならクエリを発行せずにnilを返す。
func (r *ItemRepository) LockByIDs(ctx context.Context, ids []model.ItemID) ([]*model.Item, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.q.LockItemsByIDs(ctx, itemUUIDs(ids))
	if err != nil {
		return nil, err
	}

	items := make([]*model.Item, len(rows))
	for i, row := range rows {
		items[i] = r.toModel(row)
	}

	return items, nil
}

// ListByIDs は指定したIDのアイテムを、FindByID と同じく状態を問わずにまとめて返す。並び順は決めない。
// 交換の品が指すアイテムを1回のクエリで引くのに使う。ids が空ならクエリを発行せずにnilを返す。
func (r *ItemRepository) ListByIDs(ctx context.Context, ids []model.ItemID) ([]*model.Item, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.q.ListItemsByIDs(ctx, itemUUIDs(ids))
	if err != nil {
		return nil, err
	}

	items := make([]*model.Item, len(rows))
	for i, row := range rows {
		items[i] = r.toModel(row)
	}

	return items, nil
}

// DecrementTraded は、「交換できた」で終えた交換 tradeID の品を、2人のリストから1点ずつ減らし、0点になったアイテムをリストから外す。
// 渡した人は交換の品が指す譲れるアイテムを、受け取った人は同じグッズのほしいアイテムを減らす。交換を終えるのと同じトランザクション内で使う。
func (r *ItemRepository) DecrementTraded(ctx context.Context, tradeID model.TradeID) error {
	return r.q.DecrementTradedItems(ctx, uuid.UUID(tradeID))
}

// itemUUIDs はアイテムのIDをクエリに渡すuuidの配列にする。
func itemUUIDs(ids []model.ItemID) []uuid.UUID {
	uuids := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = uuid.UUID(id)
	}

	return uuids
}

// toModel はクエリの行を model.Item に変換する。
func (r *ItemRepository) toModel(row query.Item) *model.Item {
	return &model.Item{
		ID:          model.ItemID(row.ID),
		UserID:      model.UserID(row.UserID),
		GoodsID:     model.GoodsID(row.GoodsID),
		Kind:        model.ItemKind(row.Kind),
		Status:      model.ItemStatus(row.Status),
		Quantity:    row.Quantity,
		Note:        row.Note,
		LockVersion: row.LockVersion,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
