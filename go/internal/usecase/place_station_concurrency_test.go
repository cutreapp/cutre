package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestUpdatePlacesUsecase_StationChangeWins は、駅のアーカイブ・削除が先に駅の行を保持したとき、
// 交換場所の保存がロックを待ってから駅の状態を読み直し、選べない駅として拒否することを検証する。
func TestUpdatePlacesUsecase_StationChangeWins(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"アーカイブ", "削除"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			db := testutil.GetTestDB()
			user := newCommittedUserWithRole(t, model.UserRoleUser)
			stationID := testutil.NewStationBuilder(t, db).Build()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			stationRepo := repository.NewStationRepository(db).WithTx(tx)
			var ok bool
			if change == "アーカイブ" {
				ok, err = stationRepo.Archive(ctx, stationID, 0, "閉業")
			} else {
				ok, err = stationRepo.Delete(ctx, stationID, 0)
			}
			if err != nil || !ok {
				t.Fatalf("駅の%s = (%v, %v)、成功を期待", change, ok, err)
			}

			done := make(chan error, 1)
			go func() {
				done <- newUpdatePlacesUsecase().Execute(jaContext(), usecase.UpdatePlacesInput{UserID: user.ID, StationIDs: []string{stationID.String()}})
			}()
			assertBlocked(t, done)
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if ve := model.AsValidationError(<-done); ve == nil || len(ve.Global) != 1 {
				t.Errorf("保存のエラー = %v、選べない駅の ValidationError を期待", ve)
			}
			if exists, _ := repository.NewUserStationRepository(db).ExistsByUserID(ctx, user.ID); exists {
				t.Error("選べなくなった駅を交換場所に保存した")
			}
		})
	}
}

// TestDeleteStation_PlaceWins は、交換場所の保存が駅の行を先に保持したとき、削除が待ってから
// 保存された交換場所を見つけ、アーカイブの案内で拒否することを検証する。
func TestDeleteStation_PlaceWins(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	stationID := testutil.NewStationBuilder(t, db).Build()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	id := uuid.UUID(stationID)
	if err := tx.QueryRowContext(ctx, "SELECT id FROM stations WHERE id = $1 FOR NO KEY UPDATE", id).Scan(&id); err != nil {
		t.Fatal(err)
	}
	testutil.NewUserStationBuilder(t, tx, admin.ID, stationID).Build()

	uc, _ := newDeleteStationUsecase()
	done := make(chan error, 1)
	go func() { done <- uc.Execute(jaContext(), usecase.DeleteStationInput{User: admin, StationID: stationID}) }()
	assertBlocked(t, done)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if ve := model.AsValidationError(<-done); ve == nil || len(ve.Global) != 1 {
		t.Errorf("削除のエラー = %v、参照済みの ValidationError を期待", ve)
	}
}

// TestUpdatePlacesUsecase_SameUser は、同じユーザーの保存が先の保存を待ち、古い版を拒否することを検証する。
// 異なる駅の行だけをロックすると、両方の駅が残るため、ユーザー行のロックを必要とする。
func TestUpdatePlacesUsecase_SameUser(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	user := newCommittedUserWithRole(t, model.UserRoleUser)
	first := testutil.NewStationBuilder(t, db).Build()
	second := testutil.NewStationBuilder(t, db).Build()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	userRepo := repository.NewUserRepository(db).WithTx(tx)
	if locked, err := userRepo.LockByID(ctx, user.ID); err != nil || locked == nil {
		t.Fatalf("ユーザーのロック = (%v, %v)、ユーザーを期待", locked, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- newUpdatePlacesUsecase().Execute(ctx, usecase.UpdatePlacesInput{UserID: user.ID, LockVersion: 0, StationIDs: []string{second.String()}, PlaceNote: "後の保存"})
	}()
	assertBlocked(t, done)
	if err := repository.NewUserStationRepository(db).WithTx(tx).Replace(ctx, user.ID, []model.StationID{first}); err != nil {
		t.Fatal(err)
	}
	if updated, err := userRepo.UpdatePlaces(ctx, user.ID, "先の保存", 0); err != nil || !updated {
		t.Fatalf("先の保存 = (%v, %v)、成功を期待", updated, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if ae := model.AsAppError(<-done); ae == nil || ae.Code != model.AppErrCodeConflict {
		t.Errorf("後の保存 = %v、版の競合を期待", ae)
	}
	stations, err := repository.NewStationRepository(db).ListByUserID(ctx, user.ID)
	if err != nil || len(stations) != 1 || stations[0].ID != first {
		t.Errorf("交換場所 = (%v, %v)、先に保存した駅だけを期待", stations, err)
	}
}
