package usecase_test

import (
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestCreateMessageConsentUsecase_Execute は、有効な同意が無い人 (記録が無い・やめた・古い版) の同意を、
// 今の版の新しい記録として残すことを検証する。
func TestCreateMessageConsentUsecase_Execute(t *testing.T) {
	t.Parallel()

	earlier := time.Now().Add(-time.Hour)
	tests := []struct {
		name  string
		setup func(t *testing.T, builder *testutil.MessageConsentBuilder)
	}{
		{name: "記録が無い", setup: func(*testing.T, *testutil.MessageConsentBuilder) {}},
		{name: "同意をやめた", setup: func(_ *testing.T, builder *testutil.MessageConsentBuilder) {
			builder.WithAgreedAt(earlier).WithWithdrawnAt(earlier).Build()
		}},
		{name: "古い版に同意した", setup: func(_ *testing.T, builder *testutil.MessageConsentBuilder) {
			builder.WithAgreedAt(earlier).WithVersion(model.CurrentMessageConsentVersion - 1).Build()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			repo := repository.NewMessageConsentRepository(db).WithTx(tx)
			userID := testutil.NewUserBuilder(t, tx).Build()
			tt.setup(t, testutil.NewMessageConsentBuilder(t, tx, userID))

			if err := usecase.NewCreateMessageConsentUsecase(repo).Execute(t.Context(), usecase.CreateMessageConsentInput{UserID: userID}); err != nil {
				t.Fatalf("Execute()のエラー = %v", err)
			}
			latest, err := repo.FindLatestByUserID(t.Context(), userID)
			if err != nil || latest == nil || !latest.IsValid() {
				t.Errorf("最新の同意 = (%+v, %v)、有効な同意を期待", latest, err)
			}
		})
	}
}

// TestCreateMessageConsentUsecase_Execute_Conflict は、既に有効な同意があるときは記録を増やさず、
// AppErrCodeConflict を返すことを検証する。
func TestCreateMessageConsentUsecase_Execute_Conflict(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewMessageConsentRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	existingID := testutil.NewMessageConsentBuilder(t, tx, userID).Build()

	err := usecase.NewCreateMessageConsentUsecase(repo).Execute(t.Context(), usecase.CreateMessageConsentInput{UserID: userID})
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
		t.Errorf("エラー = %v、AppErrCodeConflictを期待", err)
	}
	if latest, err := repo.FindLatestByUserID(t.Context(), userID); err != nil || latest == nil || latest.ID != existingID {
		t.Errorf("最新の同意 = (%+v, %v)、既存の同意 %s のままであることを期待", latest, err, existingID)
	}
}
