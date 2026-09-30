package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestGoodsRepository_CreateAndFindByID は、作ったグッズがカテゴリーの配下に公開中・版0でできることと、
// 無いIDには (nil, nil) を返すことを検証する。
func TestGoodsRepository_CreateAndFindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewGoodsRepository(db).WithTx(tx)
	ctx := context.Background()
	eventCategoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	created, err := repo.Create(ctx, eventCategoryID, repository.GoodsAttributes{Name: "くまの子", Position: 2})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if created.EventCategoryID != eventCategoryID || created.Status != model.MasterStatusPublished || created.LockVersion != 0 || created.ArchiveMessage != nil {
		t.Errorf("作ったグッズ = %+v、グッズ %s の配下に公開中・版0・理由なしを期待", created, eventCategoryID)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil || found == nil {
		t.Fatalf("FindByID() = (%v, %v)、グッズを期待", found, err)
	}
	if found.Name != "くまの子" || found.Position != 2 {
		t.Errorf("読み戻したグッズ = %+v、作った名前と並び順を期待", found)
	}

	missing, err := repo.FindByID(ctx, model.GoodsID(uuid.New()))
	if err != nil || missing != nil {
		t.Errorf("無いIDのFindByID() = (%v, %v)、(nil, nil) を期待", missing, err)
	}
}

// TestGoodsRepository_ListUndeletedByEventCategoryID は、指定したカテゴリーのグッズだけを、削除したものを除いて
// 並び順に並べ、同じ並び順は作った順に並べることを検証する。
func TestGoodsRepository_ListUndeletedByEventCategoryID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewGoodsRepository(db).WithTx(tx)
	eventCategoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	thirdID := testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithPosition(2).Build()
	firstID := testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithPosition(1).WithArchived("景品から外れたため").Build()
	secondID := testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithPosition(1).Build()
	testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithPosition(0).WithDeleted().Build()
	testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()

	goods, err := repo.ListUndeletedByEventCategoryID(context.Background(), eventCategoryID)
	if err != nil {
		t.Fatalf("ListUndeletedByEventCategoryID()のエラー = %v", err)
	}
	want := []model.GoodsID{firstID, secondID, thirdID}
	if len(goods) != len(want) {
		t.Fatalf("グッズの数 = %d、期待値 = %d", len(goods), len(want))
	}
	for i, item := range goods {
		if item.ID != want[i] {
			t.Errorf("%d番目のグッズ = %s、期待値 = %s", i, item.ID, want[i])
		}
	}
}

// TestGoodsRepository_Update は、版が一致するときだけ更新して版を上げ、
// 版が古いときと削除したグッズは更新しないことを検証する。
func TestGoodsRepository_Update(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewGoodsRepository(db).WithTx(tx)
	ctx := context.Background()
	eventCategoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()
	id := testutil.NewGoodsBuilder(t, tx, eventCategoryID).Build()
	attrs := repository.GoodsAttributes{Name: "新しいグッズ", Position: 5}

	updated, err := repo.Update(ctx, id, 0, attrs)
	if err != nil || !updated {
		t.Fatalf("Update() = (%v, %v)、(true, nil) を期待", updated, err)
	}
	found, _ := repo.FindByID(ctx, id)
	if found.Name != "新しいグッズ" || found.Position != 5 || found.LockVersion != 1 {
		t.Errorf("更新したグッズ = %+v、新しい属性と版1を期待", found)
	}

	if updated, err := repo.Update(ctx, id, 0, attrs); err != nil || updated {
		t.Errorf("古い版のUpdate() = (%v, %v)、(false, nil) を期待", updated, err)
	}

	deletedID := testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithDeleted().Build()
	if updated, err := repo.Update(ctx, deletedID, 0, attrs); err != nil || updated {
		t.Errorf("削除したグッズのUpdate() = (%v, %v)、(false, nil) を期待", updated, err)
	}
}

// TestGoodsRepository_ArchiveUnarchiveAndDelete は、公開中のグッズだけをアーカイブして理由を残し、
// アーカイブしたものだけを公開に戻して理由を空にし、削除では行を残すことを検証する。どれも版を上げ、版が違えば更新しない。
func TestGoodsRepository_ArchiveUnarchiveAndDelete(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewGoodsRepository(db).WithTx(tx)
	ctx := context.Background()
	id := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()

	if archived, err := repo.Archive(ctx, id, 1, "版の違う送信"); err != nil || archived {
		t.Errorf("版の違うArchive() = (%v, %v)、(false, nil) を期待", archived, err)
	}
	if archived, err := repo.Archive(ctx, id, 0, "景品から外れたため"); err != nil || !archived {
		t.Fatalf("Archive() = (%v, %v)、(true, nil) を期待", archived, err)
	}
	found, _ := repo.FindByID(ctx, id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "景品から外れたため" || found.LockVersion != 1 {
		t.Errorf("アーカイブしたグッズ = %+v、アーカイブ・理由あり・版1を期待", found)
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
		t.Errorf("元に戻したグッズ = %+v、公開中・理由なし・版2を期待", found)
	}

	if deleted, err := repo.Delete(ctx, id, 1); err != nil || deleted {
		t.Errorf("版の違うDelete() = (%v, %v)、(false, nil) を期待", deleted, err)
	}
	if deleted, err := repo.Delete(ctx, id, 2); err != nil || !deleted {
		t.Fatalf("Delete() = (%v, %v)、(true, nil) を期待", deleted, err)
	}
	found, err := repo.FindByID(ctx, id)
	if err != nil || found == nil || !found.IsDeleted() {
		t.Errorf("削除したグッズ = (%+v, %v)、削除した状態で残ることを期待", found, err)
	}
	if deleted, err := repo.Delete(ctx, id, 3); err != nil || deleted {
		t.Errorf("削除済みのDelete() = (%v, %v)、(false, nil) を期待", deleted, err)
	}
}

// TestGoodsRepository_ListPublishedByEventCategoryID は、指定したカテゴリーの公開中のグッズだけを並び順に並べることを検証する。
func TestGoodsRepository_ListPublishedByEventCategoryID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewGoodsRepository(db).WithTx(tx)
	eventCategoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	secondID := testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithPosition(2).Build()
	firstID := testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithPosition(1).Build()
	testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithPosition(0).WithArchived("景品から外れたため").Build()
	testutil.NewGoodsBuilder(t, tx, eventCategoryID).WithPosition(0).WithDeleted().Build()

	goods, err := repo.ListPublishedByEventCategoryID(context.Background(), eventCategoryID)
	if err != nil {
		t.Fatalf("ListPublishedByEventCategoryID()のエラー = %v", err)
	}
	if len(goods) != 2 || goods[0].ID != firstID || goods[1].ID != secondID {
		t.Errorf("グッズ = %+v、[%s %s] を期待", goods, firstID, secondID)
	}
}

// TestGoodsRepository_CountPublished は、公開中のカテゴリーの公開中のグッズだけを、イベントごと・カテゴリーごとに数えることを検証する。
func TestGoodsRepository_CountPublished(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewGoodsRepository(db).WithTx(tx)
	ctx := context.Background()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	archivedCategoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithArchived("景品から外れたため").Build()

	testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	testutil.NewGoodsBuilder(t, tx, categoryID).WithArchived("景品から外れたため").Build()
	testutil.NewGoodsBuilder(t, tx, archivedCategoryID).Build()

	byEvent, err := repo.CountPublishedGroupByEventID(ctx)
	if err != nil {
		t.Fatalf("CountPublishedGroupByEventID()のエラー = %v", err)
	}
	if byEvent[eventID] != 2 {
		t.Errorf("イベントのグッズの数 = %d、公開中のカテゴリーの公開中のグッズの2を期待", byEvent[eventID])
	}

	byCategory, err := repo.CountPublishedByEventIDGroupByEventCategoryID(ctx, eventID)
	if err != nil {
		t.Fatalf("CountPublishedByEventIDGroupByEventCategoryID()のエラー = %v", err)
	}
	if byCategory[categoryID] != 2 {
		t.Errorf("カテゴリーのグッズの数 = %d、公開中のグッズの2を期待", byCategory[categoryID])
	}
}
