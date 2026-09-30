package usecase_test

import (
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newWithdrawMessageConsentUsecase は、自分でトランザクションを開く WithdrawMessageConsentUsecase をテスト用のデータベースで組み立てる。
func newWithdrawMessageConsentUsecase() *usecase.WithdrawMessageConsentUsecase {
	db := testutil.GetTestDB()

	return usecase.NewWithdrawMessageConsentUsecase(
		db,
		validator.NewMessageConsentWithdrawValidator(repository.NewTradeRepository(db)),
		repository.NewMessageConsentRepository(db),
		repository.NewUserRepository(db),
	)
}

// TestWithdrawMessageConsentUsecase_Execute は、有効な同意をやめたことにし、記録は消さずに残すことを検証する。
// 終わった交換 (交換できた・取り下げ) は、同意をやめるのを止めない。
func TestWithdrawMessageConsentUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	repo := repository.NewMessageConsentRepository(db)
	userID := newCommittedUserWithRole(t, model.UserRoleUser).ID
	partnerID := newCommittedUserWithRole(t, model.UserRoleUser).ID
	consentID := testutil.NewMessageConsentBuilder(t, db, userID).Build()
	testutil.NewTradeBuilder(t, db, userID, partnerID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeBuilder(t, db, partnerID, userID).WithStatus(model.TradeStatusWithdrawn).Build()

	if err := newWithdrawMessageConsentUsecase().Execute(jaContext(), usecase.WithdrawMessageConsentInput{UserID: userID}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	latest, err := repo.FindLatestByUserID(t.Context(), userID)
	if err != nil || latest == nil || latest.ID != consentID || latest.WithdrawnAt == nil {
		t.Errorf("最新の同意 = (%+v, %v)、同じ記録 %s にやめた日時が入ることを期待", latest, err, consentID)
	}
}

// TestWithdrawMessageConsentUsecase_Execute_TradeInProgress は、申し込んだか申し込まれた進行中 (返事待ち・マッチ成立) の交換があるときは、
// 同意をやめずに *model.ValidationError を返すことを検証する。
func TestWithdrawMessageConsentUsecase_Execute_TradeInProgress(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newWithdrawMessageConsentUsecase()

	for name, build := range map[string]func(userID, partnerID model.UserID){
		"申し込んだ返事待ち": func(userID, partnerID model.UserID) {
			testutil.NewTradeBuilder(t, db, userID, partnerID).Build()
		},
		"申し込まれたマッチ成立": func(userID, partnerID model.UserID) {
			testutil.NewTradeBuilder(t, db, partnerID, userID).WithStatus(model.TradeStatusMatched).Build()
		},
	} {
		userID := newCommittedUserWithRole(t, model.UserRoleUser).ID
		testutil.NewMessageConsentBuilder(t, db, userID).Build()
		build(userID, newCommittedUserWithRole(t, model.UserRoleUser).ID)

		err := uc.Execute(jaContext(), usecase.WithdrawMessageConsentInput{UserID: userID})
		if ve := model.AsValidationError(err); ve == nil || len(ve.Global) != 1 {
			t.Errorf("%s: エラー = %v、フォーム全体のエラー1つの ValidationError を期待", name, err)
		}
		if latest, _ := repository.NewMessageConsentRepository(db).FindLatestByUserID(t.Context(), userID); latest == nil || !latest.IsValid() {
			t.Errorf("%s: 最新の同意 = %+v、有効なままを期待", name, latest)
		}
	}
}

// TestWithdrawMessageConsentUsecase_Execute_Conflict は、やめていない同意が無いとき (記録が無い・既にやめた) に、
// AppErrCodeConflict を返すことを検証する。
func TestWithdrawMessageConsentUsecase_Execute_Conflict(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := newWithdrawMessageConsentUsecase()

	withdrawnUserID := newCommittedUserWithRole(t, model.UserRoleUser).ID
	testutil.NewMessageConsentBuilder(t, db, withdrawnUserID).WithWithdrawnAt(time.Now()).Build()
	for name, userID := range map[string]model.UserID{
		"記録が無い": newCommittedUserWithRole(t, model.UserRoleUser).ID,
		"既にやめた": withdrawnUserID,
	} {
		err := uc.Execute(jaContext(), usecase.WithdrawMessageConsentInput{UserID: userID})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
			t.Errorf("%s: エラー = %v、AppErrCodeConflictを期待", name, err)
		}
	}
}
