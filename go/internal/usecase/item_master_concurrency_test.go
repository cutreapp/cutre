package usecase_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

type masterFixture struct {
	db         *sql.DB
	user       *model.User
	eventID    model.EventID
	categoryID model.EventCategoryID
	goodsID    model.GoodsID
}

func newMasterFixture(t *testing.T) masterFixture {
	t.Helper()

	db := testutil.GetTestDB()
	user := newCommittedUserWithRole(t, model.UserRoleAdmin)
	eventID := testutil.NewEventBuilder(t, db).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, db, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, db, categoryID).Build()
	return masterFixture{db: db, user: user, eventID: eventID, categoryID: categoryID, goodsID: goodsID}
}

func (f masterFixture) createItemUsecase() *usecase.CreateItemUsecase {
	return usecase.NewCreateItemUsecase(
		f.db, validator.NewItemCreateValidator(),
		repository.NewEventRepository(f.db), repository.NewEventCategoryRepository(f.db),
		repository.NewGoodsRepository(f.db), repository.NewItemRepository(f.db),
	)
}

func (f masterFixture) archive(ctx context.Context, tx *sql.Tx, master string) error {
	var ok bool
	var err error
	switch master {
	case "イベント":
		ok, err = repository.NewEventRepository(f.db).WithTx(tx).Archive(ctx, f.eventID, 0, "終了")
	case "カテゴリー":
		ok, err = repository.NewEventCategoryRepository(f.db).WithTx(tx).Archive(ctx, f.categoryID, 0, "終了")
	case "グッズ":
		ok, err = repository.NewGoodsRepository(f.db).WithTx(tx).Archive(ctx, f.goodsID, 0, "終了")
	}
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%sをアーカイブできなかった", master)
	}
	return nil
}

func (f masterFixture) markDeleted(ctx context.Context, tx *sql.Tx, master string) error {
	var ok bool
	var err error
	switch master {
	case "イベント":
		ok, err = repository.NewEventRepository(f.db).WithTx(tx).Delete(ctx, f.eventID, 0)
	case "カテゴリー":
		ok, err = repository.NewEventCategoryRepository(f.db).WithTx(tx).Delete(ctx, f.categoryID, 0)
	case "グッズ":
		ok, err = repository.NewGoodsRepository(f.db).WithTx(tx).Delete(ctx, f.goodsID, 0)
	}
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%sを削除できなかった", master)
	}
	return nil
}

func (f masterFixture) lock(ctx context.Context, tx *sql.Tx, master string) error {
	var table string
	var id uuid.UUID
	switch master {
	case "イベント":
		table, id = "events", uuid.UUID(f.eventID)
	case "カテゴリー":
		table, id = "event_categories", uuid.UUID(f.categoryID)
	case "グッズ":
		table, id = "goods", uuid.UUID(f.goodsID)
	}
	return tx.QueryRowContext(ctx, "SELECT id FROM "+table+" WHERE id = $1 FOR NO KEY UPDATE", id).Scan(&id)
}

func (f masterFixture) delete(ctx context.Context, master string) error {
	itemRepo := repository.NewItemRepository(f.db)
	eventRepo := repository.NewEventRepository(f.db)
	categoryRepo := repository.NewEventCategoryRepository(f.db)
	goodsRepo := repository.NewGoodsRepository(f.db)
	switch master {
	case "イベント":
		return usecase.NewDeleteEventUsecase(f.db, validator.NewEventDeleteValidator(itemRepo), eventRepo).Execute(ctx, usecase.DeleteEventInput{User: f.user, EventID: f.eventID})
	case "カテゴリー":
		_, err := usecase.NewDeleteEventCategoryUsecase(f.db, validator.NewEventCategoryDeleteValidator(itemRepo), eventRepo, categoryRepo).Execute(ctx, usecase.DeleteEventCategoryInput{User: f.user, EventCategoryID: f.categoryID})
		return err
	case "グッズ":
		_, err := usecase.NewDeleteGoodsUsecase(f.db, validator.NewGoodsDeleteValidator(itemRepo), eventRepo, categoryRepo, goodsRepo).Execute(ctx, usecase.DeleteGoodsInput{User: f.user, GoodsID: f.goodsID})
		return err
	}
	return fmt.Errorf("不明なマスタ: %s", master)
}

// assertBlocked は先行トランザクションがマスタ行を保持する間、操作が完了しないことを確かめる。
func assertBlocked(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("ロックの解放前に操作が完了した: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestCreateItemUsecase_MasterChangeWins は、公開状態の予備確認の後にアーカイブ・削除が確定しても、
// ロックを待った追加が状態を読み直して拒否することを検証する。
func TestCreateItemUsecase_MasterChangeWins(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"アーカイブ", "削除"} {
		for _, master := range []string{"イベント", "カテゴリー", "グッズ"} {
			t.Run(change+"/"+master, func(t *testing.T) {
				f := newMasterFixture(t)
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				tx, err := f.db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = tx.Rollback() }()
				var changeErr error
				if change == "アーカイブ" {
					changeErr = f.archive(ctx, tx, master)
				} else {
					changeErr = f.markDeleted(ctx, tx, master)
				}
				if changeErr != nil {
					t.Fatal(changeErr)
				}

				done := make(chan error, 1)
				go func() {
					_, err := f.createItemUsecase().Execute(ctx, usecase.CreateItemInput{UserID: f.user.ID, GoodsID: f.goodsID, Kind: "give", Quantity: "1"})
					done <- err
				}()
				assertBlocked(t, done)
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
				assertAppErrorCode(t, <-done, model.AppErrCodeResourceNotFound)
				var count int
				if err := f.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE goods_id = $1 AND user_id = $2", uuid.UUID(f.goodsID), uuid.UUID(f.user.ID)).Scan(&count); err != nil || count != 0 {
					t.Errorf("追加後のアイテム数 = %d、エラー = %v。0件を期待", count, err)
				}
			})
		}
	}
}

// TestDeleteMaster_ItemWins は、追加がマスタ行を先に保持したとき、削除が待ってから
// 追加済みのアイテムを発見し、アーカイブの案内で拒否することを検証する。
func TestDeleteMaster_ItemWins(t *testing.T) {
	t.Parallel()
	for _, master := range []string{"イベント", "カテゴリー", "グッズ"} {
		t.Run(master, func(t *testing.T) {
			f := newMasterFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			tx, err := f.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			if err := f.lock(ctx, tx, master); err != nil {
				t.Fatal(err)
			}
			testutil.NewItemBuilder(t, tx, f.user.ID, f.goodsID).Build()

			done := make(chan error, 1)
			go func() { done <- f.delete(ctx, master) }()
			assertBlocked(t, done)
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if ve := model.AsValidationError(<-done); ve == nil || len(ve.Global) != 1 {
				t.Errorf("削除のエラー = %v、参照済みのValidationErrorを期待", ve)
			}
		})
	}
}
