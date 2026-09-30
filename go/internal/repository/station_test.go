package repository_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestStationRepository_CreateAndFindByID は、作った駅が公開中・版0でできることと、
// 無いIDには (nil, nil) を返すことを検証する。
func TestStationRepository_CreateAndFindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewStationRepository(db).WithTx(tx)
	ctx := context.Background()

	created, err := repo.Create(ctx, repository.StationAttributes{PrefectureCode: 27, Name: "梅田", Position: 2})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if created.Status != model.MasterStatusPublished || created.LockVersion != 0 || created.ArchiveMessage != nil {
		t.Errorf("作った駅 = %+v、公開中・版0・理由なしを期待", created)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil || found == nil {
		t.Fatalf("FindByID() = (%v, %v)、駅を期待", found, err)
	}
	if found.PrefectureCode != 27 || found.Name != "梅田" || found.Position != 2 {
		t.Errorf("読み戻した駅 = %+v、作った都道府県・名前・並び順を期待", found)
	}

	missing, err := repo.FindByID(ctx, model.StationID(uuid.New()))
	if err != nil || missing != nil {
		t.Errorf("無いIDのFindByID() = (%v, %v)、(nil, nil) を期待", missing, err)
	}
}

// TestStationRepository_ListUndeleted は、削除したものを除いた駅を、都道府県コードの順、
// 都道府県の中では並び順に並べ、同じ並び順は作った順に並べることを検証する。
func TestStationRepository_ListUndeleted(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewStationRepository(db).WithTx(tx)

	fourthID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(27).WithPosition(0).Build()
	thirdID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithPosition(2).Build()
	firstID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithPosition(1).WithArchived("閉業したため").Build()
	secondID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithPosition(1).Build()
	deletedID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(1).WithDeleted().Build()
	ours := map[model.StationID]bool{firstID: true, secondID: true, thirdID: true, fourthID: true, deletedID: true}

	stations, err := repo.ListUndeleted(context.Background())
	if err != nil {
		t.Fatalf("ListUndeleted()のエラー = %v", err)
	}
	// 並行するほかのテストがコミットした駅も混ざるため、このテストで作ったものだけを順に拾う。
	var got []model.StationID
	for _, station := range stations {
		if ours[station.ID] {
			got = append(got, station.ID)
		}
	}
	want := []model.StationID{firstID, secondID, thirdID, fourthID}
	if len(got) != len(want) {
		t.Fatalf("駅の数 = %d、期待値 = %d", len(got), len(want))
	}
	for i, id := range got {
		if id != want[i] {
			t.Errorf("%d番目の駅 = %s、期待値 = %s", i, id, want[i])
		}
	}
}

// TestStationRepository_Update は、版が一致するときだけ更新して版を上げ、
// 版が古いときと削除した駅は更新しないことを検証する。
func TestStationRepository_Update(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewStationRepository(db).WithTx(tx)
	ctx := context.Background()
	id := testutil.NewStationBuilder(t, tx).Build()
	attrs := repository.StationAttributes{PrefectureCode: 14, Name: "横浜", Position: 5}

	updated, err := repo.Update(ctx, id, 0, attrs)
	if err != nil || !updated {
		t.Fatalf("Update() = (%v, %v)、(true, nil) を期待", updated, err)
	}
	found, _ := repo.FindByID(ctx, id)
	if found.PrefectureCode != 14 || found.Name != "横浜" || found.Position != 5 || found.LockVersion != 1 {
		t.Errorf("更新した駅 = %+v、新しい属性と版1を期待", found)
	}

	if updated, err := repo.Update(ctx, id, 0, attrs); err != nil || updated {
		t.Errorf("古い版のUpdate() = (%v, %v)、(false, nil) を期待", updated, err)
	}

	deletedID := testutil.NewStationBuilder(t, tx).WithDeleted().Build()
	if updated, err := repo.Update(ctx, deletedID, 0, attrs); err != nil || updated {
		t.Errorf("削除した駅のUpdate() = (%v, %v)、(false, nil) を期待", updated, err)
	}
}

// TestStationRepository_ArchiveUnarchiveAndDelete は、公開中の駅だけをアーカイブして理由を残し、
// アーカイブしたものだけを公開に戻して理由を空にし、削除では行を残すことを検証する。どれも版を上げ、版が違えば更新しない。
func TestStationRepository_ArchiveUnarchiveAndDelete(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewStationRepository(db).WithTx(tx)
	ctx := context.Background()
	id := testutil.NewStationBuilder(t, tx).Build()

	if archived, err := repo.Archive(ctx, id, 1, "版の違う送信"); err != nil || archived {
		t.Errorf("版の違うArchive() = (%v, %v)、(false, nil) を期待", archived, err)
	}
	if archived, err := repo.Archive(ctx, id, 0, "閉業したため"); err != nil || !archived {
		t.Fatalf("Archive() = (%v, %v)、(true, nil) を期待", archived, err)
	}
	found, _ := repo.FindByID(ctx, id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "閉業したため" || found.LockVersion != 1 {
		t.Errorf("アーカイブした駅 = %+v、アーカイブ・理由あり・版1を期待", found)
	}
	// 状態の条件だけで拒むことを確かめるため、今の版を送る。
	if archived, err := repo.Archive(ctx, id, 1, "もう一度"); err != nil || archived {
		t.Errorf("アーカイブ済みのArchive() = (%v, %v)、(false, nil) を期待", archived, err)
	}

	if unarchived, err := repo.Unarchive(ctx, id, 0); err != nil || unarchived {
		t.Errorf("古い版のUnarchive() = (%v, %v)、(false, nil) を期待", unarchived, err)
	}
	if unarchived, err := repo.Unarchive(ctx, id, 1); err != nil || !unarchived {
		t.Fatalf("Unarchive() = (%v, %v)、(true, nil) を期待", unarchived, err)
	}
	found, _ = repo.FindByID(ctx, id)
	if found.Status != model.MasterStatusPublished || found.ArchiveMessage != nil || found.LockVersion != 2 {
		t.Errorf("元に戻した駅 = %+v、公開中・理由なし・版2を期待", found)
	}

	if deleted, err := repo.Delete(ctx, id, 1); err != nil || deleted {
		t.Errorf("版の違うDelete() = (%v, %v)、(false, nil) を期待", deleted, err)
	}
	if deleted, err := repo.Delete(ctx, id, 2); err != nil || !deleted {
		t.Fatalf("Delete() = (%v, %v)、(true, nil) を期待", deleted, err)
	}
	found, err := repo.FindByID(ctx, id)
	if err != nil || found == nil || !found.IsDeleted() {
		t.Errorf("削除した駅 = (%+v, %v)、削除した状態で残ることを期待", found, err)
	}
	if deleted, err := repo.Delete(ctx, id, 3); err != nil || deleted {
		t.Errorf("削除済みのDelete() = (%v, %v)、(false, nil) を期待", deleted, err)
	}
}

// TestStationRepository_ListPublished は、公開中の駅だけを、都道府県コードの順、都道府県の中では並び順に並べることを検証する。
func TestStationRepository_ListPublished(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewStationRepository(db).WithTx(tx)

	thirdID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(27).WithPosition(0).Build()
	secondID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithPosition(2).Build()
	firstID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithPosition(1).Build()
	archivedID := testutil.NewStationBuilder(t, tx).WithArchived("閉業したため").Build()
	deletedID := testutil.NewStationBuilder(t, tx).WithDeleted().Build()
	ours := map[model.StationID]bool{firstID: true, secondID: true, thirdID: true, archivedID: true, deletedID: true}

	stations, err := repo.ListPublished(context.Background())
	if err != nil {
		t.Fatalf("ListPublished()のエラー = %v", err)
	}
	// 並行するほかのテストがコミットした駅も混ざるため、このテストで作ったものだけを順に拾う。
	var got []model.StationID
	for _, station := range stations {
		if ours[station.ID] {
			got = append(got, station.ID)
		}
	}
	if want := []model.StationID{firstID, secondID, thirdID}; !slices.Equal(got, want) {
		t.Errorf("駅 = %v、期待値 = %v", got, want)
	}
}

// TestStationRepository_ListByUserID は、ユーザーが交換場所に選んだ駅だけを、状態を問わずに、
// 都道府県コードの順、都道府県の中では並び順に並べることを検証する。
func TestStationRepository_ListByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewStationRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()

	secondID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(14).Build()
	firstID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithArchived("閉業したため").Build()
	testutil.NewUserStationBuilder(t, tx, userID, secondID).Build()
	testutil.NewUserStationBuilder(t, tx, userID, firstID).Build()
	testutil.NewUserStationBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), testutil.NewStationBuilder(t, tx).Build()).Build()

	stations, err := repo.ListByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListByUserID()のエラー = %v", err)
	}
	if len(stations) != 2 || stations[0].ID != firstID || stations[1].ID != secondID {
		t.Errorf("ListByUserID() = %v、アーカイブした駅を含む2駅を都道府県の順に期待", stations)
	}
}

// TestStationRepository_LockAndListByIDs は、指定したIDの駅をロックでき、状態を問わずにまとめて返し、
// 無いIDは飛ばすことと、IDが空ならクエリを発行しないことを検証する。
func TestStationRepository_LockAndListByIDs(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewStationRepository(db).WithTx(tx)
	publishedID := testutil.NewStationBuilder(t, tx).Build()
	deletedID := testutil.NewStationBuilder(t, tx).WithDeleted().Build()
	missingID := model.StationID(uuid.New())
	ids := []model.StationID{publishedID, deletedID, missingID}

	if err := repo.LockByIDs(ctx, ids); err != nil {
		t.Fatalf("LockByIDs()のエラー = %v", err)
	}
	if err := repo.LockByID(ctx, missingID); err != nil {
		t.Fatalf("無いIDのLockByID()のエラー = %v", err)
	}
	stations, err := repo.ListByIDs(ctx, ids)
	if err != nil || len(stations) != 2 {
		t.Errorf("ListByIDs() = (%v, %v)、削除した駅を含む2駅を期待", stations, err)
	}

	if err := repo.LockByIDs(ctx, nil); err != nil {
		t.Errorf("空のLockByIDs()のエラー = %v", err)
	}
	if stations, err := repo.ListByIDs(ctx, nil); err != nil || stations != nil {
		t.Errorf("空のListByIDs() = (%v, %v)、(nil, nil) を期待", stations, err)
	}
}

// TestStationRepository_ListByUserIDs は、ユーザーごとの交換場所の駅を、状態を問わずに都道府県の順でまとめて返し、
// 駅を選んでいないユーザーはmapに入れないことと、ユーザーが空ならクエリを発行しないことを検証する。
func TestStationRepository_ListByUserIDs(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewStationRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	otherUserID := testutil.NewUserBuilder(t, tx).Build()
	noPlacesUserID := testutil.NewUserBuilder(t, tx).Build()

	secondID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(14).Build()
	firstID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithArchived("閉業したため").Build()
	testutil.NewUserStationBuilder(t, tx, userID, secondID).Build()
	testutil.NewUserStationBuilder(t, tx, userID, firstID).Build()
	testutil.NewUserStationBuilder(t, tx, otherUserID, secondID).Build()

	stations, err := repo.ListByUserIDs(ctx, []model.UserID{userID, otherUserID, noPlacesUserID})
	if err != nil {
		t.Fatalf("ListByUserIDs()のエラー = %v", err)
	}
	if got := stations[userID]; len(got) != 2 || got[0].ID != firstID || got[1].ID != secondID {
		t.Errorf("ユーザーの駅 = %v、アーカイブした駅を含む2駅を都道府県の順に期待", got)
	}
	if got := stations[otherUserID]; len(got) != 1 || got[0].ID != secondID {
		t.Errorf("ほかのユーザーの駅 = %v、1駅を期待", got)
	}
	if _, ok := stations[noPlacesUserID]; ok || len(stations) != 2 {
		t.Errorf("ListByUserIDs() = %v、駅を選んだ2人だけを期待", stations)
	}

	if stations, err := repo.ListByUserIDs(ctx, nil); err != nil || stations != nil {
		t.Errorf("空のユーザーでの ListByUserIDs() = (%v, %v)、(nil, nil) を期待", stations, err)
	}
}
