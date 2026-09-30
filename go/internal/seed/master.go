package seed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// masterPositionStep はマスタの見本の並び順の間隔。書いた順に100・200・300…と振る。
// 管理画面の作成のフォームの初期値と同じ間隔にし、間に並べ替えを試せるようにする。
const masterPositionStep = 100

// Masters は開発環境へ作るマスタ (イベント・カテゴリー・グッズ・駅) の見本。
type Masters struct {
	Events   []MasterEvent
	Stations []MasterStations
}

// MasterEvent はイベント1件と、その配下のカテゴリー・グッズ。
type MasterEvent struct {
	Name     string
	StartsOn time.Time
	// EndsOn はnilなら終わりが決まっていないイベントにする。
	EndsOn     *time.Time
	Categories []MasterEventCategory
}

// MasterEventCategory はカテゴリー1件と、その配下のグッズの名前。並び順は書いた順にする。
type MasterEventCategory struct {
	Name  string
	Goods []string
}

// MasterStations は都道府県1つの駅の名前。並び順は書いた順にする。
type MasterStations struct {
	PrefectureCode model.PrefectureCode
	Names          []string
}

// DefaultMasters は開発用のマスタの見本を返す。
//
// イベントの名前は実在のくじと取り違えないよう「見本のくじ」とし、開催期間は today を基準に、
// 開催中のものと、終わりが決まっていないこれからのものを1件ずつ用意する。
// 駅は、交換場所の都道府県が同じ相手を探すマッチを試せるよう、いくつかの都道府県に分けて置く。
func DefaultMasters(today time.Time) Masters {
	date := func(days int) time.Time {
		d := today.AddDate(0, 0, days)
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	}
	autumnEndsOn := date(30)

	return Masters{
		Events: []MasterEvent{
			{
				Name:     "見本のくじ 秋",
				StartsOn: date(-7),
				EndsOn:   &autumnEndsOn,
				Categories: []MasterEventCategory{
					{Name: "A賞 ぬいぐるみ", Goods: []string{"くまの子", "うさぎの子"}},
					{Name: "B賞 ラバーマスコット", Goods: []string{"くまの子", "うさぎの子", "ねこの子", "いぬの子"}},
					{Name: "C賞 アクリルスタンド", Goods: []string{"くまの子", "うさぎの子", "ねこの子"}},
				},
			},
			{
				Name:     "見本のくじ 冬",
				StartsOn: date(30),
				Categories: []MasterEventCategory{
					{Name: "A賞 マグカップ", Goods: []string{"しろ", "くろ"}},
					{Name: "B賞 ハンドタオル", Goods: []string{"あお", "あか", "きいろ"}},
				},
			},
		},
		Stations: []MasterStations{
			{PrefectureCode: 1, Names: []string{"札幌"}},
			{PrefectureCode: 13, Names: []string{"東京", "新宿", "渋谷", "池袋", "秋葉原"}},
			{PrefectureCode: 14, Names: []string{"横浜", "川崎"}},
			{PrefectureCode: 23, Names: []string{"名古屋", "栄"}},
			{PrefectureCode: 27, Names: []string{"梅田", "難波", "天王寺"}},
			{PrefectureCode: 40, Names: []string{"博多", "天神"}},
		},
	}
}

// CreateMasters はマスタの見本のうち、まだ無いものを作る。作ったものと飛ばしたものを out に書く。
//
// 削除していない同じ名前のイベントが既にあれば、そのイベントは配下のカテゴリー・グッズごと作らない。
// 駅は、削除していない同じ都道府県・同じ名前の駅が既にあれば作らない。
// 何度実行しても同じ状態に落ち着き、管理画面で名前を変えたものは見本とは別のものとして扱う。
// イベントは1件ずつ配下と一緒にトランザクションで包み、カテゴリーの無いイベントを残さない。
func CreateMasters(ctx context.Context, db *sql.DB, masters Masters, out io.Writer) error {
	existingEvents, err := repository.NewEventRepository(db).ListUndeleted(ctx)
	if err != nil {
		return fmt.Errorf("イベントの確認に失敗: %w", err)
	}
	eventNames := map[string]bool{}
	for _, event := range existingEvents {
		eventNames[event.Name] = true
	}
	for _, event := range masters.Events {
		if eventNames[event.Name] {
			_, _ = fmt.Fprintf(out, "既にあります: イベント「%s」\n", event.Name)
			continue
		}
		if err := createEvent(ctx, db, event); err != nil {
			return fmt.Errorf("イベント「%s」の作成に失敗: %w", event.Name, err)
		}
		_, _ = fmt.Fprintf(out, "作成しました: イベント「%s」\n", event.Name)
	}

	stationRepo := repository.NewStationRepository(db)
	existingStations, err := stationRepo.ListUndeleted(ctx)
	if err != nil {
		return fmt.Errorf("駅の確認に失敗: %w", err)
	}
	type stationKey struct {
		code model.PrefectureCode
		name string
	}
	stationKeys := map[stationKey]bool{}
	for _, station := range existingStations {
		stationKeys[stationKey{code: station.PrefectureCode, name: station.Name}] = true
	}
	for _, stations := range masters.Stations {
		var position int32
		for _, name := range stations.Names {
			position += masterPositionStep
			if stationKeys[stationKey{code: stations.PrefectureCode, name: name}] {
				_, _ = fmt.Fprintf(out, "既にあります: 駅「%s」(都道府県コード %d)\n", name, stations.PrefectureCode)
				continue
			}
			if _, err := stationRepo.Create(ctx, repository.StationAttributes{
				PrefectureCode: stations.PrefectureCode,
				Name:           name,
				Position:       position,
			}); err != nil {
				return fmt.Errorf("駅「%s」の作成に失敗: %w", name, err)
			}
			_, _ = fmt.Fprintf(out, "作成しました: 駅「%s」(都道府県コード %d)\n", name, stations.PrefectureCode)
		}
	}

	return nil
}

// createEvent はイベント1件を、配下のカテゴリー・グッズと一緒に作る。
func createEvent(ctx context.Context, db *sql.DB, event MasterEvent) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	created, err := repository.NewEventRepository(db).WithTx(tx).Create(ctx, repository.EventAttributes{
		Name:     event.Name,
		StartsOn: event.StartsOn,
		EndsOn:   event.EndsOn,
	})
	if err != nil {
		return err
	}

	categoryRepo := repository.NewEventCategoryRepository(db).WithTx(tx)
	goodsRepo := repository.NewGoodsRepository(db).WithTx(tx)
	var categoryPosition int32
	for _, category := range event.Categories {
		categoryPosition += masterPositionStep
		createdCategory, err := categoryRepo.Create(ctx, created.ID, repository.EventCategoryAttributes{Name: category.Name, Position: categoryPosition})
		if err != nil {
			return err
		}
		var goodsPosition int32
		for _, name := range category.Goods {
			goodsPosition += masterPositionStep
			if _, err := goodsRepo.Create(ctx, createdCategory.ID, repository.GoodsAttributes{Name: name, Position: goodsPosition}); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
