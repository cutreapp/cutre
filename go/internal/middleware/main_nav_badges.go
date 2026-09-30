package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
)

// MainNavBadgesLoader は、ユーザー userID のメインメニューに出す数字を引く。
// データの引き方はミドルウェアの関心事ではないため、UseCaseを呼ぶ関数を配線の側で渡す。
type MainNavBadgesLoader func(ctx context.Context, userID model.UserID) (templates.MainNavBadges, error)

// MainNavBadges はメインメニューの数字を引くミドルウェアの依存を保持する。
type MainNavBadges struct {
	load MainNavBadgesLoader
}

// NewMainNavBadges は MainNavBadges を生成する。
func NewMainNavBadges(load MainNavBadgesLoader) *MainNavBadges {
	return &MainNavBadges{load: load}
}

// Middleware は、ログイン中のユーザーのメインメニューに出す数字を引き、リクエストcontextに載せる。
// RequireAuth の内側に掛け、ログインを求めるルートだけに適用する。
//
// メインメニューはログイン後のすべてのページに出るため、ハンドラーごとに引かずここで1回引く。
// 引けなかったときはログに残して数字の無いまま先へ進める。数字のためにページ全体を落とさないようにするため。
func (m *MainNavBadges) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		user := UserFromContext(ctx)
		if user == nil {
			next.ServeHTTP(w, r)
			return
		}

		badges, err := m.load(ctx, user.ID)
		if err != nil {
			slog.WarnContext(ctx, "メインメニューの数字の取得に失敗しました", "error", err, "user_id", user.ID)
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(templates.WithMainNavBadges(ctx, badges)))
	})
}
