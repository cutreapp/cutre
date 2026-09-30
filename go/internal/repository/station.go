package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// StationRepository はstations (駅) を読み書きする。
type StationRepository struct {
	q *query.Queries
}

// NewStationRepository は StationRepository を生成する。
func NewStationRepository(db *sql.DB) *StationRepository {
	return &StationRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい StationRepository を返す。
func (r *StationRepository) WithTx(tx *sql.Tx) *StationRepository {
	return &StationRepository{q: r.q.WithTx(tx)}
}

// FindByID は指定したIDの駅を状態を問わずに返す。存在しない場合は (nil, nil) を返す。
// 削除した駅を存在しないものとして扱うかは呼び出し側が決める。
func (r *StationRepository) FindByID(ctx context.Context, id model.StationID) (*model.Station, error) {
	row, err := r.q.GetStationByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// ListUndeleted は削除していない (公開中とアーカイブした) 駅を、都道府県コードの順、都道府県の中では並び順に返す。
func (r *StationRepository) ListUndeleted(ctx context.Context) ([]*model.Station, error) {
	rows, err := r.q.ListUndeletedStations(ctx)
	if err != nil {
		return nil, err
	}

	stations := make([]*model.Station, len(rows))
	for i, row := range rows {
		stations[i] = r.toModel(row)
	}

	return stations, nil
}

// LockByID は駅の行をトランザクションの終了までロックする。
// 無い駅でもエラーにしない。有無と状態は、呼び出し側がロックの取得後に別の文で読み直して判定する。
func (r *StationRepository) LockByID(ctx context.Context, id model.StationID) error {
	if _, err := r.q.LockStationByID(ctx, uuid.UUID(id)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// LockByIDs は指定したIDの駅の行を、idの順にトランザクションの終了までロックする。
// 無い駅は飛ばす。有無と状態は、呼び出し側がロックの取得後に別の文で読み直して判定する。
func (r *StationRepository) LockByIDs(ctx context.Context, ids []model.StationID) error {
	if len(ids) == 0 {
		return nil
	}

	_, err := r.q.LockStationsByIDs(ctx, stationUUIDs(ids))
	return err
}

// ListByIDs は指定したIDの駅を、FindByID と同じく状態を問わずにまとめて返す。並び順は決めない。
// ids が空ならクエリを発行せずにnilを返す。
func (r *StationRepository) ListByIDs(ctx context.Context, ids []model.StationID) ([]*model.Station, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.q.ListStationsByIDs(ctx, stationUUIDs(ids))
	if err != nil {
		return nil, err
	}

	return r.toModels(rows), nil
}

// ListPublished は公開中の駅を、都道府県コードの順、都道府県の中では並び順に返す。交換場所の選択肢に使う。
func (r *StationRepository) ListPublished(ctx context.Context) ([]*model.Station, error) {
	rows, err := r.q.ListPublishedStations(ctx)
	if err != nil {
		return nil, err
	}

	return r.toModels(rows), nil
}

// ListByUserID はユーザーが交換場所に選んだ駅を、状態を問わずに、都道府県コードの順、都道府県の中では並び順に返す。
func (r *StationRepository) ListByUserID(ctx context.Context, userID model.UserID) ([]*model.Station, error) {
	rows, err := r.q.ListStationsByUserID(ctx, uuid.UUID(userID))
	if err != nil {
		return nil, err
	}

	return r.toModels(rows), nil
}

// ListByUserIDs は、ユーザー userIDs が交換場所に選んだ駅を、ユーザーごとにまとめて返す。
// ListByUserID と同じく状態を問わず、ユーザーごとに都道府県コードの順、都道府県の中では並び順に並べる。
// 駅を選んでいないユーザーはmapに入らない。userIDs が空ならクエリを発行せずにnilを返す。
func (r *StationRepository) ListByUserIDs(ctx context.Context, userIDs []model.UserID) (map[model.UserID][]*model.Station, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	rows, err := r.q.ListStationsByUserIDs(ctx, userUUIDs(userIDs))
	if err != nil {
		return nil, err
	}

	stations := make(map[model.UserID][]*model.Station)
	for _, row := range rows {
		userID := model.UserID(row.UserID)
		stations[userID] = append(stations[userID], r.toModel(row.Station))
	}

	return stations, nil
}

// StationAttributes は管理画面の編集のフォームで決める駅の属性。
type StationAttributes struct {
	PrefectureCode model.PrefectureCode
	Name           string
	Position       int32
}

// Create は公開中の駅を挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *StationRepository) Create(ctx context.Context, attrs StationAttributes) (*model.Station, error) {
	row, err := r.q.CreateStation(ctx, query.CreateStationParams{
		PrefectureCode: int16(attrs.PrefectureCode),
		Name:           attrs.Name,
		Position:       attrs.Position,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// Update は、駅の版が lockVersion と一致するときだけ属性を更新し、版を1つ上げる。
// 版が一致しない (ほかの操作で先に更新された) か、削除していて更新しなかったときはfalseを返す。
func (r *StationRepository) Update(ctx context.Context, id model.StationID, lockVersion int32, attrs StationAttributes) (bool, error) {
	affected, err := r.q.UpdateStation(ctx, query.UpdateStationParams{
		ID:             uuid.UUID(id),
		PrefectureCode: int16(attrs.PrefectureCode),
		Name:           attrs.Name,
		Position:       attrs.Position,
		LockVersion:    lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Archive は公開中の駅を、理由 archiveMessage を残してアーカイブする。
// 公開中でないか版が一致せず更新しなかったときはfalseを返す。
func (r *StationRepository) Archive(ctx context.Context, id model.StationID, lockVersion int32, archiveMessage string) (bool, error) {
	affected, err := r.q.ArchiveStation(ctx, query.ArchiveStationParams{
		ID:             uuid.UUID(id),
		ArchiveMessage: &archiveMessage,
		LockVersion:    lockVersion,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Unarchive はアーカイブした駅を公開に戻し、理由を空にする。
// アーカイブしていないか版が一致せず更新しなかったときはfalseを返す。
func (r *StationRepository) Unarchive(ctx context.Context, id model.StationID, lockVersion int32) (bool, error) {
	affected, err := r.q.UnarchiveStation(ctx, query.UnarchiveStationParams{ID: uuid.UUID(id), LockVersion: lockVersion})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Delete は駅を削除した状態にする。行は消さない。
// 既に削除しているか版が一致せず更新しなかったときはfalseを返す。
func (r *StationRepository) Delete(ctx context.Context, id model.StationID, lockVersion int32) (bool, error) {
	affected, err := r.q.DeleteStation(ctx, query.DeleteStationParams{ID: uuid.UUID(id), LockVersion: lockVersion})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// toModels はクエリの行をまとめて model.Station に変換する。
func (r *StationRepository) toModels(rows []query.Station) []*model.Station {
	stations := make([]*model.Station, len(rows))
	for i, row := range rows {
		stations[i] = r.toModel(row)
	}

	return stations
}

// stationUUIDs は駅のIDをクエリに渡すuuidの配列にする。
func stationUUIDs(ids []model.StationID) []uuid.UUID {
	uuids := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = uuid.UUID(id)
	}

	return uuids
}

// toModel はクエリの行を model.Station に変換する。
func (r *StationRepository) toModel(row query.Station) *model.Station {
	return &model.Station{
		ID:             model.StationID(row.ID),
		PrefectureCode: model.PrefectureCode(row.PrefectureCode),
		Name:           row.Name,
		Position:       row.Position,
		Status:         model.MasterStatus(row.Status),
		ArchiveMessage: row.ArchiveMessage,
		LockVersion:    row.LockVersion,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
