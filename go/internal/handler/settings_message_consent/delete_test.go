package settings_message_consent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// withdraw は handler の Delete を、POSTに _method を載せたリクエストで MethodOverride を通して呼ぶ。
func withdraw(handler http.HandlerFunc, user *model.User) *httptest.ResponseRecorder {
	form := url.Values{"_method": {http.MethodDelete}}
	req := httptest.NewRequest(http.MethodPost, "/settings/message_consent", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	middleware.MethodOverride(handler).ServeHTTP(rec, withUser(req, user))
	return rec
}

// newCommittedUser はコミットしたテスト用のユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
// 同意をやめるUseCaseは自分でトランザクションを開くため、テストのトランザクションの中の行を読めない。
func newCommittedUser(t *testing.T) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	id := testutil.NewUserBuilder(t, db).Build()
	user, err := repository.NewUserRepository(db).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// latestConsent はユーザーの最新の同意の記録を返す。
func latestConsent(t *testing.T, userID model.UserID) *model.MessageConsent {
	t.Helper()

	consent, err := repository.NewMessageConsentRepository(testutil.GetTestDB()).FindLatestByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("同意の取得に失敗しました: %v", err)
	}

	return consent
}

// TestDelete は、有効な同意をやめ、完了のメッセージを付けてメッセージの利用の画面へ戻すことを検証する。
// 終わった交換は、同意をやめるのを止めない。
func TestDelete(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	db := testutil.GetTestDB()
	user := newCommittedUser(t)
	testutil.NewMessageConsentBuilder(t, db, user.ID).Build()
	testutil.NewTradeBuilder(t, db, user.ID, newCommittedUser(t).ID).WithStatus(model.TradeStatusCompleted).Build()

	rec := withdraw(handler.Delete, user)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" {
		t.Fatalf("応答 = %d %q、303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if !flashSet(rec) {
		t.Error("完了のメッセージが書き込まれていない")
	}
	if consent := latestConsent(t, user.ID); consent == nil || consent.WithdrawnAt == nil {
		t.Errorf("最新の同意 = %+v、やめた同意を期待", consent)
	}
}

// TestDelete_TradeInProgress は、進行中の交換があるときは同意をやめず、そのことを伝えてメッセージの利用の画面を描き直すことを検証する。
func TestDelete_TradeInProgress(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	db := testutil.GetTestDB()
	user := newCommittedUser(t)
	testutil.NewMessageConsentBuilder(t, db, user.ID).Build()
	testutil.NewTradeBuilder(t, db, newCommittedUser(t).ID, user.ID).Build()

	rec := withdraw(handler.Delete, user)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), "進行中の交換があるため、同意をやめられません。", `role="alert"`)
	if flashSet(rec) {
		t.Error("同意をやめていないのに、完了のメッセージを書き込んだ")
	}
	if consent := latestConsent(t, user.ID); consent == nil || !consent.IsValid() {
		t.Errorf("最新の同意 = %+v、有効なままを期待", consent)
	}
}

// TestDelete_NotAgreed は、やめていない同意が無いときは、完了のメッセージを付けずに戻すことを検証する。
func TestDelete_NotAgreed(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	user := newCommittedUser(t)

	rec := withdraw(handler.Delete, user)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" {
		t.Fatalf("応答 = %d %q、303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if flashSet(rec) {
		t.Error("同意をやめていないのに、完了のメッセージを書き込んだ")
	}
}

// TestDelete_WithoutUser は、RequireAuth を通さずに届いたリクエストで誰の同意もやめないことを検証する。
func TestDelete_WithoutUser(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	rec := withdraw(handler.Delete, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
