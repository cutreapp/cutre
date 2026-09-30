package repository_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestUserStationRepository_Replace は、ユーザーの交換場所を指定した駅だけに置き換え、
// ほかのユーザーの交換場所には触れず、空にすれば交換場所をすべて外すことを検証する。
func TestUserStationRepository_Replace(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserStationRepository(db).WithTx(tx)
	stationRepo := repository.NewStationRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	otherUserID := testutil.NewUserBuilder(t, tx).Build()
	first := testutil.NewStationBuilder(t, tx).WithPosition(1).Build()
	second := testutil.NewStationBuilder(t, tx).WithPosition(2).Build()
	third := testutil.NewStationBuilder(t, tx).WithPosition(3).Build()
	testutil.NewUserStationBuilder(t, tx, otherUserID, first).Build()

	selected := func(id model.UserID) []model.StationID {
		t.Helper()
		stations, err := stationRepo.ListByUserID(ctx, id)
		if err != nil {
			t.Fatalf("ListByUserID()のエラー = %v", err)
		}
		ids := make([]model.StationID, len(stations))
		for i, station := range stations {
			ids[i] = station.ID
		}
		return ids
	}

	if err := repo.Replace(ctx, userID, []model.StationID{first, second}); err != nil {
		t.Fatalf("Replace()のエラー = %v", err)
	}
	if got := selected(userID); len(got) != 2 || got[0] != first || got[1] != second {
		t.Errorf("交換場所 = %v、1番目と2番目の駅を期待", got)
	}

	if err := repo.Replace(ctx, userID, []model.StationID{second, third}); err != nil {
		t.Fatalf("Replace()のエラー = %v", err)
	}
	if got := selected(userID); len(got) != 2 || got[0] != second || got[1] != third {
		t.Errorf("交換場所 = %v、2番目と3番目の駅を期待", got)
	}

	if err := repo.Replace(ctx, userID, nil); err != nil {
		t.Fatalf("空のReplace()のエラー = %v", err)
	}
	if got := selected(userID); len(got) != 0 {
		t.Errorf("交換場所 = %v、空を期待", got)
	}
	if got := selected(otherUserID); len(got) != 1 {
		t.Errorf("ほかのユーザーの交換場所 = %v、そのままを期待", got)
	}
}

// TestUserStationRepository_DeleteByUserID は、ユーザーの交換場所をすべて消し、ほかのユーザーの交換場所には触れないことを検証する。
func TestUserStationRepository_DeleteByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserStationRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	otherUserID := testutil.NewUserBuilder(t, tx).Build()
	first := testutil.NewStationBuilder(t, tx).WithPosition(1).Build()
	second := testutil.NewStationBuilder(t, tx).WithPosition(2).Build()
	testutil.NewUserStationBuilder(t, tx, userID, first).Build()
	testutil.NewUserStationBuilder(t, tx, userID, second).Build()
	testutil.NewUserStationBuilder(t, tx, otherUserID, first).Build()

	if err := repo.DeleteByUserID(ctx, userID); err != nil {
		t.Fatalf("DeleteByUserID()のエラー = %v", err)
	}
	if exists, err := repo.ExistsByUserID(ctx, userID); err != nil || exists {
		t.Errorf("消したあとのExistsByUserID() = (%v, %v)、falseを期待", exists, err)
	}
	if exists, err := repo.ExistsByUserID(ctx, otherUserID); err != nil || !exists {
		t.Errorf("ほかのユーザーのExistsByUserID() = (%v, %v)、trueを期待", exists, err)
	}
}

// TestUserStationRepository_Exists は、ユーザーが交換場所を選んでいるかと、駅を交換場所に選んでいるユーザーがいるかを返すことを検証する。
func TestUserStationRepository_Exists(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewUserStationRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	stationID := testutil.NewStationBuilder(t, tx).Build()

	if exists, err := repo.ExistsByUserID(ctx, userID); err != nil || exists {
		t.Errorf("選ぶ前のExistsByUserID() = (%v, %v)、falseを期待", exists, err)
	}
	if exists, err := repo.ExistsByStationID(ctx, stationID); err != nil || exists {
		t.Errorf("選ぶ前のExistsByStationID() = (%v, %v)、falseを期待", exists, err)
	}

	testutil.NewUserStationBuilder(t, tx, userID, stationID).Build()

	if exists, err := repo.ExistsByUserID(ctx, userID); err != nil || !exists {
		t.Errorf("選んだあとのExistsByUserID() = (%v, %v)、trueを期待", exists, err)
	}
	if exists, err := repo.ExistsByStationID(ctx, stationID); err != nil || !exists {
		t.Errorf("選んだあとのExistsByStationID() = (%v, %v)、trueを期待", exists, err)
	}
}
