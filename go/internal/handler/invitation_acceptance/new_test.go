package invitation_acceptance_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/invitation_acceptance"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

const testContinuationKey = "test-continuation-token-key-0123456789"

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// newRouter はテスト用のトランザクションの中で動く Handler を、本番と同じパスの形で登録したルーターを返す。
// パスの {token} を読むにはchiのルーティングを通す必要があるため、ハンドラーを直接呼ばずにルーター経由で呼ぶ。
// ロケールはパスの言語版に合わせてcontextへ載せる (本番ではI18nのミドルウェアが行う)。
func newRouter(t *testing.T) (http.Handler, *sql.Tx) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	handler := invitation_acceptance.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewContinuationManager(testContinuationKey),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db).WithTx(tx)),
		usecase.NewGetInvitationUsecase(
			repository.NewInvitationRepository(db).WithTx(tx),
			repository.NewInvitationRedemptionRepository(db).WithTx(tx),
			repository.NewUserRepository(db).WithTx(tx),
		),
	)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			locale := i18n.LangJa
			if strings.HasPrefix(req.URL.Path, "/en/") {
				locale = i18n.LangEn
			}
			next.ServeHTTP(w, req.WithContext(i18n.SetLocale(req.Context(), locale)))
		})
	})
	for _, prefix := range []string{"", "/en"} {
		r.Get(prefix+"/i/{token}", handler.New)
		r.Post(prefix+"/i/{token}", handler.Create)
	}

	return r, tx
}

// serve はリクエストを送り、応答を返す。remoteAddrはレート制限を数える単位を他のテストと分けるために渡す。
func serve(router http.Handler, method, target, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

// TestNew は、使える招待では招待した人と登録を始めるフォームを描画し、インデックスを断ることを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	router, tx := newRouter(t)
	atname := testutil.UniqueAtname()
	inviterID := testutil.NewUserBuilder(t, tx).WithAtname(atname).Build()
	invitation := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID)
	invitation.Build()

	tests := []struct {
		name         string
		target       string
		wantContains []string
	}{
		{
			name:   "日本語版",
			target: "/i/" + invitation.Token(),
			wantContains: []string{
				`<html lang="ja">`,
				"@" + atname + "から招待が届いています。",
				`action="/i/` + invitation.Token() + `"`,
				`name="csrf_token"`,
				`<meta name="robots" content="noindex">`,
			},
		},
		{
			name:         "英語版",
			target:       "/en/i/" + invitation.Token(),
			wantContains: []string{`<html lang="en">`, "@" + atname + " has invited you.", `action="/en/i/` + invitation.Token() + `"`},
		},
	}

	for _, tt := range tests {
		rec := serve(router, http.MethodGet, tt.target, "192.0.2.10:1234")

		if rec.Code != http.StatusOK {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		for _, want := range tt.wantContains {
			if !strings.Contains(body, want) {
				t.Errorf("%s: レスポンスボディに %q が含まれていない", tt.name, want)
			}
		}
		if strings.Contains(body, `rel="canonical"`) {
			t.Errorf("%s: インデックスを断るページがcanonicalを宣言している", tt.name)
		}
	}
}

// TestNew_Unusable は、使えない招待では登録を始めるフォームを出さずに404で示すことを検証する。
func TestNew_Unusable(t *testing.T) {
	t.Parallel()

	router, tx := newRouter(t)
	expired := testutil.NewInvitationBuilder(t, tx).WithExpiresAt(time.Now().Add(-time.Minute))
	expired.Build()

	for _, token := range []string{expired.Token(), "no-such-invitation"} {
		rec := serve(router, http.MethodGet, "/i/"+token, "192.0.2.11:1234")

		if rec.Code != http.StatusNotFound {
			t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "この招待リンクは使えません") {
			t.Error("使えない招待の説明が含まれていない")
		}
		if strings.Contains(body, `method="post"`) {
			t.Error("使えない招待で登録を始めるフォームが描画されている")
		}
	}
}

// TestNew_RateLimited は、同じIPアドレスから上限を超えて開くと、招待が使えるかどうかを示さずに429を返すことを検証する。
func TestNew_RateLimited(t *testing.T) {
	t.Parallel()

	router, tx := newRouter(t)
	invitation := testutil.NewInvitationBuilder(t, tx)
	invitation.Build()

	const remoteAddr = "192.0.2.12:1234"
	for i := range 30 {
		if rec := serve(router, http.MethodGet, "/i/no-such-invitation", remoteAddr); rec.Code != http.StatusNotFound {
			t.Fatalf("%d回目のステータスコード = %d、期待値 = %d", i+1, rec.Code, http.StatusNotFound)
		}
	}

	rec := serve(router, http.MethodGet, "/i/"+invitation.Token(), remoteAddr)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が付いていない")
	}
	if strings.Contains(rec.Body.String(), `method="post"`) {
		t.Error("上限を超えたリクエストに登録を始めるフォームが描画されている")
	}
}
