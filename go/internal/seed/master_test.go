package seed_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/seed"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestDefaultMasters は、見本の開催期間を today から決め、駅の都道府県コードがどれも都道府県を指すことを検証する。
func TestDefaultMasters(t *testing.T) {
	t.Parallel()

	today := time.Date(2026, 9, 28, 23, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	masters := seed.DefaultMasters(today)

	if len(masters.Events) == 0 || len(masters.Stations) == 0 {
		t.Fatalf("見本 = %+v、イベントと駅を期待", masters)
	}
	autumn := masters.Events[0]
	if !autumn.StartsOn.Equal(time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)) || autumn.EndsOn == nil || !autumn.EndsOn.Equal(time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("開催中の見本の期間 = %v〜%v、2026-09-21〜2026-10-28を期待", autumn.StartsOn, autumn.EndsOn)
	}
	for _, stations := range masters.Stations {
		if _, ok := model.FindPrefecture(stations.PrefectureCode); !ok {
			t.Errorf("都道府県コード %d が都道府県を指していない", stations.PrefectureCode)
		}
	}
}

// TestCreateMasters は、見本のイベントを配下のカテゴリー・グッズと一緒に、駅を都道府県ごとに間を空けた並び順で作り、
// 再実行しても重複して作られないことを検証する。
//
// CreateMasters は自前でトランザクションを開くため、共有の接続で実行して行をコミットする。
// 名前は実行ごとに一意にし、他のテストと衝突しないようにする。
func TestCreateMasters(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	ctx := context.Background()
	suffix := uuid.NewString()
	masters := seed.Masters{
		Events: []seed.MasterEvent{{
			Name:     "見本のテスト " + suffix,
			StartsOn: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			Categories: []seed.MasterEventCategory{
				{Name: "A賞", Goods: []string{"くまの子", "うさぎの子"}},
			},
		}},
		Stations: []seed.MasterStations{{PrefectureCode: 46, Names: []string{"駅A " + suffix, "駅B " + suffix}}},
	}

	var out strings.Builder
	if err := seed.CreateMasters(ctx, db, masters, &out); err != nil {
		t.Fatalf("CreateMasters()のエラー = %v", err)
	}

	event := findEventByName(t, masters.Events[0].Name)
	categories, err := repository.NewEventCategoryRepository(db).ListUndeletedByEventID(ctx, event.ID)
	if err != nil || len(categories) != 1 || categories[0].Name != "A賞" || categories[0].Position != 100 {
		t.Fatalf("カテゴリー = (%v, %v)、並び順100の「A賞」を1つ期待", categories, err)
	}
	goods, err := repository.NewGoodsRepository(db).ListUndeletedByEventCategoryID(ctx, categories[0].ID)
	if err != nil || len(goods) != 2 || goods[0].Name != "くまの子" || goods[1].Position != 200 {
		t.Errorf("グッズ = (%v, %v)、「くまの子」と並び順200の「うさぎの子」を期待", goods, err)
	}
	positions := map[string]int32{}
	stations, err := repository.NewStationRepository(db).ListUndeleted(ctx)
	if err != nil {
		t.Fatalf("ListUndeleted()のエラー = %v", err)
	}
	for _, station := range stations {
		if strings.HasSuffix(station.Name, suffix) {
			positions[station.Name] = station.Position
		}
	}
	if positions["駅A "+suffix] != 100 || positions["駅B "+suffix] != 200 {
		t.Errorf("駅の並び順 = %v、100と200を期待", positions)
	}

	out.Reset()
	if err := seed.CreateMasters(ctx, db, masters, &out); err != nil {
		t.Fatalf("2回目のCreateMasters()のエラー = %v", err)
	}
	if got := strings.Count(out.String(), "既にあります"); got != 3 {
		t.Errorf("飛ばしたマスタ = %d件、期待値 = 3件\n出力: %s", got, out.String())
	}
	if strings.Contains(out.String(), "作成しました") {
		t.Errorf("2回目に作成したマスタがある\n出力: %s", out.String())
	}
}

// findEventByName は削除していないイベントから name のものを探す。無ければテストを止める。
func findEventByName(t *testing.T, name string) *model.Event {
	t.Helper()

	events, err := repository.NewEventRepository(testutil.GetTestDB()).ListUndeleted(context.Background())
	if err != nil {
		t.Fatalf("ListUndeleted()のエラー = %v", err)
	}
	for _, event := range events {
		if event.Name == name {
			return event
		}
	}
	t.Fatalf("イベント「%s」が作られていない", name)

	return nil
}
