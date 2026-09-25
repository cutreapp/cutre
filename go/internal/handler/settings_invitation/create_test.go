package settings_invitation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// create はログイン中のユーザーとして招待リンクを作り直す。
func create(user *model.User, invitationID string) *httptest.ResponseRecorder {
	form := url.Values{"invitation_id": {invitationID}}
	req := httptest.NewRequest(http.MethodPost, "/settings/invitation", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := i18n.SetLocale(req.Context(), string(user.Locale))
	ctx = middleware.SetUserToContext(ctx, user)
	rec := httptest.NewRecorder()
	newHandler().Create(rec, req.WithContext(ctx))

	return rec
}

// flashSet は応答がフラッシュメッセージを設定したかを返す。
func flashSet(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.FlashCookieName && c.Value != "" {
			return true
		}
	}

	return false
}

// TestCreate は、今の招待を取り消して新しい招待を作り、完了を伝えて招待の画面へ戻すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	currentID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()

	rec := create(user, currentID.String())

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/invitation" {
		t.Fatalf("応答 = %d %q、期待値 = 303 /settings/invitation", rec.Code, rec.Header().Get("Location"))
	}
	if !flashSet(rec) {
		t.Error("フラッシュメッセージが設定されていない")
	}

	invitationRepo := repository.NewInvitationRepository(db)
	previous, err := invitationRepo.FindByID(ctx, currentID)
	if err != nil || previous == nil || previous.RevokedAt == nil {
		t.Errorf("今までの招待 = (%+v, %v)、取り消されていることを期待", previous, err)
	}
	recreated, err := invitationRepo.FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || recreated == nil || recreated.ID == currentID {
		t.Errorf("取り消していない招待 = (%+v, %v)、今までとは別の招待を期待", recreated, err)
	}
}

// TestCreate_Full は、招待できる人数の上限に達していれば、作り直さずに完了も伝えず招待の画面へ戻すことを検証する。
func TestCreate_Full(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	invitationID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()
	for range model.InviterRedemptionLimit {
		testutil.NewInvitationRedemptionBuilder(t, db, invitationID).Build()
	}

	rec := create(user, invitationID.String())

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/invitation" {
		t.Fatalf("応答 = %d %q、期待値 = 303 /settings/invitation", rec.Code, rec.Header().Get("Location"))
	}
	if flashSet(rec) {
		t.Error("作り直していないのにフラッシュメッセージが設定されている")
	}

	current, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(context.Background(), user.ID)
	if err != nil || current == nil || current.ID != invitationID {
		t.Errorf("取り消していない招待 = (%+v, %v)、今までの招待 %v のままを期待", current, err, invitationID)
	}
}

// TestCreate_InvalidInvitationID は、不正な招待IDで作り直さず、404で応えることを検証する。
func TestCreate_InvalidInvitationID(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	rec := create(user, "invalid")
	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestCreate_Duplicate は、同じ画面からの送信を繰り返しても最初に作った招待を取り消さず、
// 作り直していない2件目では完了を伝えないことを検証する。
func TestCreate_Duplicate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	currentID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()
	if rec := create(user, currentID.String()); rec.Code != http.StatusSeeOther {
		t.Fatalf("1件目のステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	first, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || first == nil {
		t.Fatalf("1件目の招待 = (%+v, %v)、新しい招待を期待", first, err)
	}
	rec := create(user, currentID.String())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("2件目のステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if flashSet(rec) {
		t.Error("作り直していない2件目でフラッシュメッセージが設定されている")
	}
	second, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || second == nil || second.ID != first.ID {
		t.Errorf("2件目の招待 = (%+v, %v)、1件目の招待 %v を期待", second, err, first.ID)
	}
}

// TestCreate_OtherInvitation は、別のユーザーが作った招待IDを指定しても取り消さず、404で応えることを検証する。
func TestCreate_OtherInvitation(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	other := signedInUser(t, model.LocaleJa)
	otherID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(other.ID).Build()
	rec := create(user, otherID.String())
	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	invitation, err := repository.NewInvitationRepository(db).FindByID(context.Background(), otherID)
	if err != nil || invitation == nil || invitation.RevokedAt != nil {
		t.Errorf("別のユーザーの招待 = (%+v, %v)、取り消されていないことを期待", invitation, err)
	}
}

// TestCreate_WithoutUser は、RequireAuth を通さずに届いたリクエストで招待を作り直さないことを検証する。
func TestCreate_WithoutUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/settings/invitation", nil)
	rec := httptest.NewRecorder()
	newHandler().Create(rec, req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
