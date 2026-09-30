package settings_message_consent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// flashSet は応答が完了のメッセージのCookieを書き込んだかを返す。
func flashSet(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.FlashCookieName && c.Value != "" {
			return true
		}
	}

	return false
}

// TestCreate は、同意をやめた人の同意を記録し、完了のメッセージを付けてメッセージの利用の画面へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	user := newUser(t, tx)
	testutil.NewMessageConsentBuilder(t, tx, user.ID).WithAgreedAt(time.Now().Add(-time.Hour)).WithWithdrawnAt(time.Now().Add(-time.Hour)).Build()

	rec := httptest.NewRecorder()
	handler.Create(rec, withUser(httptest.NewRequest(http.MethodPost, "/settings/message_consent", nil), user))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" {
		t.Fatalf("応答 = %d %q、303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if !flashSet(rec) {
		t.Error("完了のメッセージが書き込まれていない")
	}
	consent, err := repository.NewMessageConsentRepository(testutil.GetTestDB()).WithTx(tx).FindLatestByUserID(context.Background(), user.ID)
	if err != nil || consent == nil || !consent.IsValid() {
		t.Errorf("最新の同意 = (%+v, %v)、有効な同意を期待", consent, err)
	}
}

// TestCreate_AlreadyAgreed は、既に有効な同意があるときは記録を増やさず、完了のメッセージを付けずに戻すことを検証する。
func TestCreate_AlreadyAgreed(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	user := newUser(t, tx)
	existingID := testutil.NewMessageConsentBuilder(t, tx, user.ID).Build()

	rec := httptest.NewRecorder()
	handler.Create(rec, withUser(httptest.NewRequest(http.MethodPost, "/settings/message_consent", nil), user))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" {
		t.Fatalf("応答 = %d %q、303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if flashSet(rec) {
		t.Error("同意を記録していないのに、完了のメッセージを書き込んだ")
	}
	consent, err := repository.NewMessageConsentRepository(testutil.GetTestDB()).WithTx(tx).FindLatestByUserID(context.Background(), user.ID)
	if err != nil || consent == nil || consent.ID != existingID {
		t.Errorf("最新の同意 = (%+v, %v)、既存の同意 %s のままであることを期待", consent, err, existingID)
	}
}

// TestCreate_WithoutUser は、RequireAuth を通さずに届いたリクエストで誰の同意も記録しないことを検証する。
func TestCreate_WithoutUser(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Create(rec, withUser(httptest.NewRequest(http.MethodPost, "/settings/message_consent", nil), nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
