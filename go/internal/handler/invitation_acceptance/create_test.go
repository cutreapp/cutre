package invitation_acceptance_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestCreate は、使える招待をCookieに移して、表示中の言語版の登録の画面へ送ることを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	router, tx := newRouter(t)

	tests := []struct {
		name         string
		prefix       string
		wantLocation string
	}{
		{name: "日本語版", prefix: "", wantLocation: "/sign_up"},
		{name: "英語版", prefix: "/en", wantLocation: "/en/sign_up"},
	}

	for _, tt := range tests {
		invitation := testutil.NewInvitationBuilder(t, tx)
		id := invitation.Build()

		rec := serve(router, http.MethodPost, tt.prefix+"/i/"+invitation.Token(), "192.0.2.20:1234")

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.wantLocation {
			t.Fatalf("%s: 応答 = %d %q、期待値 = 303 %q", tt.name, rec.Code, rec.Header().Get("Location"), tt.wantLocation)
		}

		// 書き込んだCookieを次のリクエストに載せ、同じ鍵で招待のIDとして読み戻せることを確かめる。
		req := httptest.NewRequest(http.MethodGet, tt.wantLocation, nil)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
		got, ok := session.NewContinuationManager(testContinuationKey).InvitationID(req)
		if !ok || got != id {
			t.Errorf("%s: Cookieの招待のID = (%v, %t)、期待値 = (%v, true)", tt.name, got, ok, id)
		}
	}
}

// TestCreate_Unusable は、表示のあとに使えなくなった招待では、Cookieを発行せずに404で示すことを検証する。
func TestCreate_Unusable(t *testing.T) {
	t.Parallel()

	router, tx := newRouter(t)
	revoked := testutil.NewInvitationBuilder(t, tx).WithRevokedAt(time.Now())
	revoked.Build()

	rec := serve(router, http.MethodPost, "/i/"+revoked.Token(), "192.0.2.21:1234")

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("Cookie = %v、発行しないことを期待", cookies)
	}
}
