package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
)

// TestMainNavBadges は、ログイン中のユーザーのメインメニューの数字を引いてcontextに載せ、
// 未ログインのときと引けなかったときは数字の無いまま通すことを検証する。
func TestMainNavBadges(t *testing.T) {
	t.Parallel()

	user := &model.User{ID: model.UserID(uuid.New())}
	tests := []struct {
		name       string
		user       *model.User
		loadErr    error
		wantLoaded bool
		want       int64
	}{
		{name: "ログイン中は数字を載せる", user: user, wantLoaded: true, want: 3},
		{name: "未ログインなら引かずに通す", user: nil},
		{name: "引けなかったら数字の無いまま通す", user: user, loadErr: errors.New("接続できません"), wantLoaded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loaded := false
			badges := middleware.NewMainNavBadges(func(_ context.Context, userID model.UserID) (templates.MainNavBadges, error) {
				loaded = true
				if userID != user.ID {
					t.Errorf("引いたユーザー = %v、期待値 = %v", userID, user.ID)
				}
				return templates.MainNavBadges{AwaitingTradeCount: 3}, tt.loadErr
			})
			var got templates.MainNavBadges
			served := false
			handler := badges.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				served = true
				got = templates.MainNavBadgesFromContext(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			if tt.user != nil {
				req = req.WithContext(middleware.SetUserToContext(req.Context(), tt.user))
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)

			if !served {
				t.Fatal("後続のハンドラーに届かなかった")
			}
			if loaded != tt.wantLoaded {
				t.Errorf("数字を引いたか = %v、期待値 = %v", loaded, tt.wantLoaded)
			}
			if got.AwaitingTradeCount != tt.want {
				t.Errorf("AwaitingTradeCount = %d、期待値 = %d", got.AwaitingTradeCount, tt.want)
			}
		})
	}
}
