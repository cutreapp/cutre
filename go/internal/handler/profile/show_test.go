package profile_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/profile"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newRequest は、ルーターがパスから取り出したアットネームを持つリクエストを作る。
func newRequest(ctx context.Context, atname string, user *model.User) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/@"+atname, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("atname", atname)
	ctx = context.WithValue(i18n.SetLocale(ctx, i18n.LangEn), chi.RouteCtxKey, routeCtx)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}
	return req.WithContext(ctx)
}

// newHandler はテスト用のトランザクションの中で動く Handler を返す。
func newHandler(t *testing.T) (*profile.Handler, *sql.Tx) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	getInvitationRedemptionsUC := usecase.NewGetInvitationRedemptionsUsecase(repository.NewInvitationRedemptionRepository(db).WithTx(tx))
	getTwoFactorAuthStatusUC := usecase.NewGetTwoFactorAuthStatusUsecase(
		repository.NewUserTwoFactorAuthRepository(db).WithTx(tx),
		repository.NewUserTwoFactorRecoveryCodeRepository(db).WithTx(tx),
	)

	getMessageConsentUC := usecase.NewGetMessageConsentUsecase(repository.NewMessageConsentRepository(db).WithTx(tx))
	getPlacesUC := usecase.NewGetPlacesUsecase(repository.NewStationRepository(db).WithTx(tx), repository.NewUserRepository(db).WithTx(tx))

	getProfileUC := usecase.NewGetProfileUsecase(
		repository.NewEventCategoryRepository(db).WithTx(tx),
		repository.NewGoodsRepository(db).WithTx(tx),
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewStationRepository(db).WithTx(tx),
		repository.NewTradeRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
	getEndedTradeCountsUC := usecase.NewGetEndedTradeCountsUsecase(repository.NewTradeRepository(db).WithTx(tx))

	return profile.NewHandler(
		cfg, httperror.NewRenderer(cfg), getInvitationRedemptionsUC, getTwoFactorAuthStatusUC, getMessageConsentUC, getPlacesUC, getProfileUC, getEndedTradeCountsUC,
	), tx
}

// TestShow は、自分のプロフィールが招待の画面への入口 (残りの人数と参加した人数)・交換場所の画面への入口 (未設定)・
// メッセージの利用の画面への入口 (未同意)・
// 二要素認証の画面への入口 (オフのバッジ付き)・「交換できた」の件数 (0件)・これまでの交換の画面への入口 (終わった交換なし)・
// ログアウトのフォーム (DELETEに上書きしたPOST) を持ち、
// 言語版を持たないページとしてcanonicalも別言語版への参照も宣言せず、インデックスも断ることを検証する。
// アットネームは大文字小文字を区別しないため、綴りの大小が違っても自分のプロフィールとして描く。
func TestShow(t *testing.T) {
	t.Parallel()

	for _, atname := range []string{"cutre_user", "Cutre_User"} {
		t.Run(atname, func(t *testing.T) {
			t.Parallel()

			const csrfToken = "profile-csrf-token"
			handler, tx := newHandler(t)
			userID := testutil.NewUserBuilder(t, tx).Build()
			invitationID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(userID).Build()
			testutil.NewInvitationRedemptionBuilder(t, tx, invitationID).Build()
			testutil.NewInvitationRedemptionBuilder(t, tx, invitationID).Build()

			req := newRequest(t.Context(), atname, &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn})
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
			rec := httptest.NewRecorder()
			middleware.NewCSRF().Middleware(http.HandlerFunc(handler.Show)).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			for _, want := range []string{
				`<html lang="en" data-main-nav>`,
				"<title>My page | Cutre</title>",
				`<h1 class="text-2xl font-bold">My page</h1>`,
				`aria-current="page"`,
				`action="/user_session" method="post"`,
				`name="_method" value="DELETE"`,
				`name="csrf_token" value="` + csrfToken + `"`,
				"Sign out",
				`href="/settings/invitation"`,
				"Invitations",
				"Invites left: 3 · Joined: 2",
				`href="/settings/places"`,
				"Trade spots",
				"Not added yet",
				`href="/settings/message_consent"`,
				"Messaging",
				"Not agreed · Required to propose or accept trades",
				`href="/settings/two_factor_auth"`,
				"Two-factor authentication",
				`<span class="badge" data-variant="outline">Off</span>`,
				"Completed trades",
				`<dd class="text-lg font-semibold">0</dd>`,
				`href="/trades/history"`,
				"Past trades",
				"None yet",
				`href="/settings/withdrawal"`,
				"Delete account",
				`<meta name="robots" content="noindex">`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
			for _, absent := range []string{`rel="canonical"`, `rel="alternate"`, `rel="preconnect"`} {
				if strings.Contains(body, absent) {
					t.Errorf("レスポンスボディに %q が含まれている", absent)
				}
			}
		})
	}
}

// TestShow_TradeHistory は、マイページに「交換できた」の件数と、これまでの交換の画面への入口に終わった交換の段階ごとの数を出すことを検証する。
func TestShow_TradeHistory(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeBuilder(t, tx, partnerID, userID).WithStatus(model.TradeStatusCancelled).Build()
	testutil.NewTradeBuilder(t, tx, userID, partnerID).WithStatus(model.TradeStatusMatched).Build()

	req := newRequest(t.Context(), "cutre_user", &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn})
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{`<dd class="text-lg font-semibold">2</dd>`, "Completed 2, Cancelled 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// TestShow_TwoFactorAuthEnabled は、二要素認証を有効にしていれば、二要素認証の入口にオンのバッジを付けることを検証する。
// 認証アプリへの登録の途中の設定では、まだオフとして描く。
func TestShow_TwoFactorAuthEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		enabled   bool
		wantBadge string
	}{
		{name: "有効", enabled: true, wantBadge: `<span class="badge" data-variant="success">On</span>`},
		{name: "登録の途中", enabled: false, wantBadge: `<span class="badge" data-variant="outline">Off</span>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, tx := newHandler(t)
			userID := testutil.NewUserBuilder(t, tx).Build()
			builder := testutil.NewUserTwoFactorAuthBuilder(t, tx, userID)
			if tt.enabled {
				builder.WithEnabledAt(time.Now())
			}
			builder.Build()

			rec := httptest.NewRecorder()
			handler.Show(rec, newRequest(t.Context(), "cutre_user", &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn}))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBadge) {
				t.Errorf("レスポンスボディに %q が含まれていない", tt.wantBadge)
			}
		})
	}
}

// TestShow_MessageConsent は、メッセージの利用の入口に、有効な同意があるかを添えることを検証する。
// やめた同意と古い版の文面への同意は、有効な同意として扱わない。
func TestShow_MessageConsent(t *testing.T) {
	t.Parallel()

	const agreed = "Agreed"
	const notAgreed = "Not agreed · Required to propose or accept trades"
	tests := []struct {
		name  string
		setup func(builder *testutil.MessageConsentBuilder)
		want  string
	}{
		{name: "有効な同意", setup: func(builder *testutil.MessageConsentBuilder) { builder.Build() }, want: agreed},
		{name: "やめた同意", setup: func(builder *testutil.MessageConsentBuilder) { builder.WithWithdrawnAt(time.Now()).Build() }, want: notAgreed},
		{name: "古い版の同意", setup: func(builder *testutil.MessageConsentBuilder) {
			builder.WithVersion(model.CurrentMessageConsentVersion - 1).Build()
		}, want: notAgreed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, tx := newHandler(t)
			userID := testutil.NewUserBuilder(t, tx).Build()
			tt.setup(testutil.NewMessageConsentBuilder(t, tx, userID))

			rec := httptest.NewRecorder()
			handler.Show(rec, newRequest(t.Context(), "cutre_user", &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn}))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			want := `<span class="text-sm text-muted-foreground">` + tt.want + `</span>`
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("レスポンスボディに %q が含まれていない", want)
			}
		})
	}
}

// TestShow_Places は、交換場所の入口に、選んだ駅の名前を都道府県の順・並び順につないで添えることを検証する。
func TestShow_Places(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	for _, station := range []struct {
		code     model.PrefectureCode
		name     string
		position int32
	}{{14, "川崎", 1}, {13, "渋谷", 2}, {13, "新宿", 1}} {
		stationID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(station.code).WithName(station.name).WithPosition(station.position).Build()
		testutil.NewUserStationBuilder(t, tx, userID, stationID).Build()
	}

	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t.Context(), "cutre_user", &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn}))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	want := `<span class="text-sm text-muted-foreground">新宿・渋谷・川崎</span>`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("レスポンスボディに %q が含まれていない", want)
	}
}

// TestShow_AdminLink は、管理画面への行を編集者と管理者にだけ出すことを検証する。
func TestShow_AdminLink(t *testing.T) {
	t.Parallel()

	for role, want := range map[model.UserRole]bool{model.UserRoleUser: false, model.UserRoleEditor: true, model.UserRoleAdmin: true} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()

			handler, tx := newHandler(t)
			userID := testutil.NewUserBuilder(t, tx).Build()

			rec := httptest.NewRecorder()
			handler.Show(rec, newRequest(t.Context(), "cutre_user", &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn, Role: role}))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := strings.Contains(rec.Body.String(), `href="/admin"`); got != want {
				t.Errorf("管理画面への行がある = %v、期待値 = %v", got, want)
			}
		})
	}
}

// TestShow_Other は、ほかの人のアットネームならその人のプロフィールとして、交換場所・「ほかに出られるところ」・「交換できた」の件数と、
// 見ているユーザーとの間で交換できるもの (あなたとの交換) を出し、マイページのメニューは出さないことを検証する。
// アットネームは大文字小文字を区別しない。
func TestShow_Other(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).WithName("B賞").Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).WithName("りすの子").Build()
	viewerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewItemBuilder(t, tx, viewerID, goodsID).WithKind(model.ItemKindWant).Build()

	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, tx).WithAtname(atname).Build()
	for _, name := range []string{"新宿", "渋谷"} {
		testutil.NewUserStationBuilder(t, tx, userID, testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithName(name).Build()).Build()
	}
	testutil.NewItemBuilder(t, tx, userID, goodsID).Build()
	testutil.NewTradeBuilder(t, tx, userID, testutil.NewUserBuilder(t, tx).Build()).WithStatus(model.TradeStatusCompleted).Build()
	userRepo := repository.NewUserRepository(testutil.GetTestDB()).WithTx(tx)
	if updated, err := userRepo.UpdatePlaces(t.Context(), userID, "平日の夜なら\n<b>新宿</b>まで", 0); err != nil || !updated {
		t.Fatalf("UpdatePlaces() = (%v, %v)、成功を期待", updated, err)
	}

	req := newRequest(t.Context(), strings.ToUpper(atname), &model.User{ID: viewerID, Atname: "cutre_user", Locale: model.LocaleEn})
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<title>Profile | Cutre</title>",
		`<h1 class="text-xl font-semibold">@` + atname + `</h1>`,
		`href="/trades" class="block`,
		`href="/matches" class="block`,
		"<span>Tokyo 新宿・渋谷</span>",
		"Other places they can go",
		"平日の夜なら\n&lt;b&gt;新宿&lt;/b&gt;まで",
		"Completed trades",
		`<dd class="text-lg font-semibold">1</dd>`,
		"Trading with you",
		"You get · Qty 1",
		"B賞 · りすの子",
		"You give · Qty 0",
		"None right now",
		`<meta name="robots" content="noindex">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	// 渡せるものが無く申し込めないため、組み合わせを選ぶ画面への入口を出さない。
	for _, absent := range []string{"Sign out", `href="/settings/invitation"`, "/trades/new"} {
		if strings.Contains(body, absent) {
			t.Errorf("レスポンスボディに %q が含まれている", absent)
		}
	}
}

// TestShow_OtherTradable は、もらえるものと渡せるものがどちらもあるほかのユーザーのプロフィールに、
// 交換を申し込む組み合わせを選ぶ画面への入口と、申し込むとメッセージでやり取りできることの案内を出すことを検証する。
func TestShow_OtherTradable(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()
	wantedGoods := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	offeredGoods := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	viewerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewItemBuilder(t, tx, viewerID, wantedGoods).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, tx, viewerID, offeredGoods).Build()
	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, tx).WithAtname(atname).Build()
	testutil.NewItemBuilder(t, tx, userID, wantedGoods).Build()
	testutil.NewItemBuilder(t, tx, userID, offeredGoods).WithKind(model.ItemKindWant).Build()

	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t.Context(), atname, &model.User{ID: viewerID, Atname: "cutre_user", Locale: model.LocaleEn}))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	for _, want := range []string{
		`<a href="/@` + atname + `/trades/new" class="btn w-full" aria-describedby="profile-propose-hint">Offer a trade</a>`,
		`<p id="profile-propose-hint" class="text-center text-sm">After you offer a trade, you can message @` + atname + `.</p>`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// TestShow_OtherNotFound は、いないアットネームと、退会したユーザーのアットネームを404にすることを検証する。
func TestShow_OtherNotFound(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	withdrawnAtname := testutil.UniqueAtname()
	testutil.NewUserBuilder(t, tx).WithAtname(withdrawnAtname).WithDeletedAt(time.Now()).Build()

	for name, atname := range map[string]string{"いない": "someone_else", "退会した": withdrawnAtname} {
		rec := httptest.NewRecorder()
		handler.Show(rec, newRequest(t.Context(), atname, &model.User{ID: testutil.NewUserBuilder(t, tx).Build(), Atname: "cutre_user", Locale: model.LocaleEn}))

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
		if strings.Contains(rec.Body.String(), "Sign out") {
			t.Errorf("%s: 404のページにログアウトのフォームが含まれている", name)
		}
	}
}

// TestShow_WithdrawnUser は、認証後に退会が完了したユーザーのプロフィールを404にすることを検証する。
func TestShow_WithdrawnUser(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	userRepo := repository.NewUserRepository(testutil.GetTestDB()).WithTx(tx)
	if withdrawn, err := userRepo.Withdraw(t.Context(), userID, model.AnonymizedEmail(userID), model.AnonymizedAtname(userID)); err != nil || !withdrawn {
		t.Fatalf("退会 = (%v, %v)、成功を期待", withdrawn, err)
	}

	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t.Context(), "cutre_user", &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn}))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かのプロフィールとして描画しないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t.Context(), "cutre_user", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
