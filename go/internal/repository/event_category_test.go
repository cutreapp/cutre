package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestEventCategoryRepository_CreateAndFindByID は、作ったカテゴリーがイベントの配下に公開中・版0でできることと、
// 無いIDには (nil, nil) を返すことを検証する。
func TestEventCategoryRepository_CreateAndFindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventCategoryRepository(db).WithTx(tx)
	ctx := context.Background()
	eventID := testutil.NewEventBuilder(t, tx).Build()

	created, err := repo.Create(ctx, eventID, repository.EventCategoryAttributes{Name: "B賞 ラバーマスコット", Position: 2})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if created.EventID != eventID || created.Status != model.MasterStatusPublished || created.LockVersion != 0 || created.ArchiveMessage != nil {
		t.Errorf("作ったカテゴリー = %+v、イベント %s の配下に公開中・版0・理由なしを期待", created, eventID)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil || found == nil {
		t.Fatalf("FindByID() = (%v, %v)、カテゴリーを期待", found, err)
	}
	if found.Name != "B賞 ラバーマスコット" || found.Position != 2 {
		t.Errorf("読み戻したカテゴリー = %+v、作った名前と並び順を期待", found)
	}

	missing, err := repo.FindByID(ctx, model.EventCategoryID(uuid.New()))
	if err != nil || missing != nil {
		t.Errorf("無いIDのFindByID() = (%v, %v)、(nil, nil) を期待", missing, err)
	}
}

// TestEventCategoryRepository_ListUndeletedByEventID は、指定したイベントのカテゴリーだけを、削除したものを除いて
// 並び順に並べ、同じ並び順は作った順に並べることを検証する。
func TestEventCategoryRepository_ListUndeletedByEventID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventCategoryRepository(db).WithTx(tx)
	eventID := testutil.NewEventBuilder(t, tx).Build()

	thirdID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(2).Build()
	firstID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(1).WithArchived("景品が変わったため").Build()
	secondID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(1).Build()
	testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(0).WithDeleted().Build()
	testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	categories, err := repo.ListUndeletedByEventID(context.Background(), eventID)
	if err != nil {
		t.Fatalf("ListUndeletedByEventID()のエラー = %v", err)
	}
	want := []model.EventCategoryID{firstID, secondID, thirdID}
	if len(categories) != len(want) {
		t.Fatalf("カテゴリーの数 = %d、期待値 = %d", len(categories), len(want))
	}
	for i, category := range categories {
		if category.ID != want[i] {
			t.Errorf("%d番目のカテゴリー = %s、期待値 = %s", i, category.ID, want[i])
		}
	}
}

// TestEventCategoryRepository_Update は、版が一致するときだけ更新して版を上げ、
// 版が古いときと削除したカテゴリーは更新しないことを検証する。
func TestEventCategoryRepository_Update(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventCategoryRepository(db).WithTx(tx)
	ctx := context.Background()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	id := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	attrs := repository.EventCategoryAttributes{Name: "新しいカテゴリー", Position: 5}

	updated, err := repo.Update(ctx, id, 0, attrs)
	if err != nil || !updated {
		t.Fatalf("Update() = (%v, %v)、(true, nil) を期待", updated, err)
	}
	found, _ := repo.FindByID(ctx, id)
	if found.Name != "新しいカテゴリー" || found.Position != 5 || found.LockVersion != 1 {
		t.Errorf("更新したカテゴリー = %+v、新しい属性と版1を期待", found)
	}

	if updated, err := repo.Update(ctx, id, 0, attrs); err != nil || updated {
		t.Errorf("古い版のUpdate() = (%v, %v)、(false, nil) を期待", updated, err)
	}

	deletedID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithDeleted().Build()
	if updated, err := repo.Update(ctx, deletedID, 0, attrs); err != nil || updated {
		t.Errorf("削除したカテゴリーのUpdate() = (%v, %v)、(false, nil) を期待", updated, err)
	}
}

// TestEventCategoryRepository_ArchiveUnarchiveAndDelete は、公開中のカテゴリーだけをアーカイブして理由を残し、
// アーカイブしたものだけを公開に戻して理由を空にし、削除では行を残すことを検証する。どれも版を上げ、版が違えば更新しない。
func TestEventCategoryRepository_ArchiveUnarchiveAndDelete(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventCategoryRepository(db).WithTx(tx)
	ctx := context.Background()
	id := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	if archived, err := repo.Archive(ctx, id, 1, "版の違う送信"); err != nil || archived {
		t.Errorf("版の違うArchive() = (%v, %v)、(false, nil) を期待", archived, err)
	}
	if archived, err := repo.Archive(ctx, id, 0, "景品が変わったため"); err != nil || !archived {
		t.Fatalf("Archive() = (%v, %v)、(true, nil) を期待", archived, err)
	}
	found, _ := repo.FindByID(ctx, id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "景品が変わったため" || found.LockVersion != 1 {
		t.Errorf("アーカイブしたカテゴリー = %+v、アーカイブ・理由あり・版1を期待", found)
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
		t.Errorf("元に戻したカテゴリー = %+v、公開中・理由なし・版2を期待", found)
	}

	if deleted, err := repo.Delete(ctx, id, 1); err != nil || deleted {
		t.Errorf("版の違うDelete() = (%v, %v)、(false, nil) を期待", deleted, err)
	}
	if deleted, err := repo.Delete(ctx, id, 2); err != nil || !deleted {
		t.Fatalf("Delete() = (%v, %v)、(true, nil) を期待", deleted, err)
	}
	found, err := repo.FindByID(ctx, id)
	if err != nil || found == nil || !found.IsDeleted() {
		t.Errorf("削除したカテゴリー = (%+v, %v)、削除した状態で残ることを期待", found, err)
	}
	if deleted, err := repo.Delete(ctx, id, 3); err != nil || deleted {
		t.Errorf("削除済みのDelete() = (%v, %v)、(false, nil) を期待", deleted, err)
	}
}

// TestEventCategoryRepository_ListPublishedByEventID は、指定したイベントの公開中のカテゴリーだけを並び順に並べることを検証する。
func TestEventCategoryRepository_ListPublishedByEventID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventCategoryRepository(db).WithTx(tx)
	eventID := testutil.NewEventBuilder(t, tx).Build()

	secondID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(2).Build()
	firstID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(1).Build()
	testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(0).WithArchived("景品から外れたため").Build()
	testutil.NewEventCategoryBuilder(t, tx, eventID).WithPosition(0).WithDeleted().Build()
	testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	categories, err := repo.ListPublishedByEventID(context.Background(), eventID)
	if err != nil {
		t.Fatalf("ListPublishedByEventID()のエラー = %v", err)
	}
	if len(categories) != 2 || categories[0].ID != firstID || categories[1].ID != secondID {
		t.Errorf("カテゴリー = %+v、[%s %s] を期待", categories, firstID, secondID)
	}
}
