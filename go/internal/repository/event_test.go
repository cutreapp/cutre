package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// date はテストで使う暦日 (UTCの0時) を返す。
func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// TestEventRepository_CreateAndFindByID は、作ったイベントが公開中・版0で、開催期間を暦日のまま読み戻せることと、
// 無いIDには (nil, nil) を返すことを検証する。
func TestEventRepository_CreateAndFindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	ctx := context.Background()

	endsOn := date(2026, 10, 31)
	created, err := repo.Create(ctx, repository.EventAttributes{Name: "秋のくじ", StartsOn: date(2026, 10, 1), EndsOn: &endsOn})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if created.Status != model.MasterStatusPublished || created.LockVersion != 0 || created.ArchiveMessage != nil {
		t.Errorf("作ったイベント = %+v、公開中・版0・理由なしを期待", created)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil || found == nil {
		t.Fatalf("FindByID() = (%v, %v)、イベントを期待", found, err)
	}
	if found.Name != "秋のくじ" || !found.StartsOn.Equal(date(2026, 10, 1)) || found.EndsOn == nil || !found.EndsOn.Equal(endsOn) {
		t.Errorf("読み戻したイベント = %+v、作った名前と開催期間を期待", found)
	}

	missing, err := repo.FindByID(ctx, model.EventID(uuid.New()))
	if err != nil || missing != nil {
		t.Errorf("無いIDのFindByID() = (%v, %v)、(nil, nil) を期待", missing, err)
	}
}

// TestEventRepository_ListUndeleted は、削除したイベントを除き、開始日の新しい順に並べることを検証する。
func TestEventRepository_ListUndeleted(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)

	olderID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 1, 1), nil).Build()
	newerID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 2, 1), nil).WithArchived("終わったため").Build()
	deletedID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 3, 1), nil).WithDeleted().Build()

	events, err := repo.ListUndeleted(context.Background())
	if err != nil {
		t.Fatalf("ListUndeleted()のエラー = %v", err)
	}
	// 並行するほかのテストがコミットしたイベントも混ざるため、このテストで作ったものだけを順に拾う。
	var got []model.EventID
	for _, event := range events {
		if event.ID == olderID || event.ID == newerID || event.ID == deletedID {
			got = append(got, event.ID)
		}
	}
	if len(got) != 2 || got[0] != newerID || got[1] != olderID {
		t.Errorf("一覧のイベント = %v、[%s %s] を期待", got, newerID, olderID)
	}
}

// TestEventRepository_Update は、版が一致するときだけ更新して版を上げ、
// 版が古いときと削除したイベントは更新しないことを検証する。
func TestEventRepository_Update(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	ctx := context.Background()
	id := testutil.NewEventBuilder(t, tx).Build()
	attrs := repository.EventAttributes{Name: "新しい名前", StartsOn: date(2026, 11, 1)}

	updated, err := repo.Update(ctx, id, 0, attrs)
	if err != nil || !updated {
		t.Fatalf("Update() = (%v, %v)、(true, nil) を期待", updated, err)
	}
	found, _ := repo.FindByID(ctx, id)
	if found.Name != "新しい名前" || !found.StartsOn.Equal(date(2026, 11, 1)) || found.EndsOn != nil || found.LockVersion != 1 {
		t.Errorf("更新したイベント = %+v、新しい属性と版1を期待", found)
	}

	if updated, err := repo.Update(ctx, id, 0, attrs); err != nil || updated {
		t.Errorf("古い版のUpdate() = (%v, %v)、(false, nil) を期待", updated, err)
	}

	deletedID := testutil.NewEventBuilder(t, tx).WithDeleted().Build()
	if updated, err := repo.Update(ctx, deletedID, 0, attrs); err != nil || updated {
		t.Errorf("削除したイベントのUpdate() = (%v, %v)、(false, nil) を期待", updated, err)
	}
}

// TestEventRepository_ArchiveAndUnarchive は、公開中のイベントだけをアーカイブして理由を残し、
// アーカイブしたものだけを公開に戻して理由を空にすることを検証する。どちらも版を上げ、版が違えば更新しない。
func TestEventRepository_ArchiveAndUnarchive(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	ctx := context.Background()
	id := testutil.NewEventBuilder(t, tx).Build()

	if archived, err := repo.Archive(ctx, id, 1, "版の違う送信"); err != nil || archived {
		t.Errorf("版の違うArchive() = (%v, %v)、(false, nil) を期待", archived, err)
	}
	if archived, err := repo.Archive(ctx, id, 0, "開催が終わったため"); err != nil || !archived {
		t.Fatalf("Archive() = (%v, %v)、(true, nil) を期待", archived, err)
	}
	found, _ := repo.FindByID(ctx, id)
	if !found.IsArchived() || found.ArchiveMessage == nil || *found.ArchiveMessage != "開催が終わったため" || found.LockVersion != 1 {
		t.Errorf("アーカイブしたイベント = %+v、アーカイブ・理由あり・版1を期待", found)
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
		t.Errorf("元に戻したイベント = %+v、公開中・理由なし・版2を期待", found)
	}
	if unarchived, err := repo.Unarchive(ctx, id, 2); err != nil || unarchived {
		t.Errorf("公開中のUnarchive() = (%v, %v)、(false, nil) を期待", unarchived, err)
	}
}

// TestEventRepository_Delete は、行を残したまま削除した状態にし、版が違うときと2度目は更新しないことを検証する。
func TestEventRepository_Delete(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	ctx := context.Background()
	id := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()

	if deleted, err := repo.Delete(ctx, id, 1); err != nil || deleted {
		t.Errorf("版の違うDelete() = (%v, %v)、(false, nil) を期待", deleted, err)
	}
	if deleted, err := repo.Delete(ctx, id, 0); err != nil || !deleted {
		t.Fatalf("Delete() = (%v, %v)、(true, nil) を期待", deleted, err)
	}
	found, err := repo.FindByID(ctx, id)
	if err != nil || found == nil || !found.IsDeleted() {
		t.Errorf("削除したイベント = (%+v, %v)、削除した状態で残ることを期待", found, err)
	}
	// 状態の条件だけで拒むことを確かめるため、今の版を送る。
	if deleted, err := repo.Delete(ctx, id, 1); err != nil || deleted {
		t.Errorf("削除済みのDelete() = (%v, %v)、(false, nil) を期待", deleted, err)
	}
}

// TestEventRepository_ListPublished は、公開中のイベントだけを開始日の新しい順に並べることを検証する。
func TestEventRepository_ListPublished(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)

	olderID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 1, 1), nil).Build()
	newerID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 2, 1), nil).Build()
	archivedID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 3, 1), nil).WithArchived("終わったため").Build()
	deletedID := testutil.NewEventBuilder(t, tx).WithPeriod(date(2001, 4, 1), nil).WithDeleted().Build()

	events, err := repo.ListPublished(context.Background())
	if err != nil {
		t.Fatalf("ListPublished()のエラー = %v", err)
	}
	// 並行するほかのテストがコミットしたイベントも混ざるため、このテストで作ったものだけを順に拾う。
	var got []model.EventID
	for _, event := range events {
		if event.ID == olderID || event.ID == newerID || event.ID == archivedID || event.ID == deletedID {
			got = append(got, event.ID)
		}
	}
	if len(got) != 2 || got[0] != newerID || got[1] != olderID {
		t.Errorf("一覧のイベント = %v、[%s %s] を期待", got, newerID, olderID)
	}
}
