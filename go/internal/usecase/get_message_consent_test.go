package usecase_test

import (
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetMessageConsentUsecase_Execute は、最新の同意の記録を返し、記録が無いときはnilを返すことを検証する。
func TestGetMessageConsentUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetMessageConsentUsecase(repository.NewMessageConsentRepository(db).WithTx(tx))

	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewMessageConsentBuilder(t, tx, userID).WithAgreedAt(time.Now().Add(-time.Hour)).Build()
	latestID := testutil.NewMessageConsentBuilder(t, tx, userID).Build()

	output, err := uc.Execute(t.Context(), usecase.GetMessageConsentInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Consent == nil || output.Consent.ID != latestID {
		t.Errorf("同意 = %+v、ID %s を期待", output.Consent, latestID)
	}

	output, err = uc.Execute(t.Context(), usecase.GetMessageConsentInput{UserID: testutil.NewUserBuilder(t, tx).Build()})
	if err != nil || output.Consent != nil {
		t.Errorf("記録の無いユーザーの結果 = (%+v, %v)、同意がnilであることを期待", output, err)
	}
}
