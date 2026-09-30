package home_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/home"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newHandler はテストのトランザクションで読む Handler を組み立てる。
func newHandler(db *sql.DB, tx *sql.Tx) *home.Handler {
	return home.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		usecase.NewGetHomeUsecase(repository.NewItemRepository(db).WithTx(tx), repository.NewUserRepository(db).WithTx(tx), repository.NewUserStationRepository(db).WithTx(tx)),
	)
}

// TestShow は、ホームがログイン中のユーザーを名指しし、譲れる・ほしいのリストの数量をそれぞれのリストへのリンクで出すことと、
// 言語版を持たないページとしてcanonicalも別言語版への参照も宣言せず、インデックスも断ることを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	handler := newHandler(db, tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithQuantity(12).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithKind(model.ItemKindWant).WithQuantity(8).Build()

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	ctx := i18n.SetLocale(req.Context(), i18n.LangEn)
	ctx = middleware.SetUserToContext(ctx, &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleEn})
	rec := httptest.NewRecorder()

	handler.Show(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{`<html lang="en" data-main-nav>`, "<title>Home | Cutre</title>", `<h1 class="text-2xl font-bold">Home</h1>`, "Welcome, @cutre_user", `href="/list?kind=give"`, "Can give list", ">12</span>", `href="/list?kind=want"`, "Want list", ">8</span>", `href="/@cutre_user"`, "My page", `<meta name="robots" content="noindex">`} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	for _, absent := range []string{`rel="canonical"`, `rel="alternate"`, `rel="preconnect"`} {
		if strings.Contains(body, absent) {
			t.Errorf("レスポンスボディに %q が含まれている", absent)
		}
	}
}

// TestShow_Setup は、交換場所をまだ選んでいないあいだだけ、交換場所の画面へ案内する準備のカードを出すことを検証する。
func TestShow_Setup(t *testing.T) {
	t.Parallel()

	for name, hasPlaces := range map[string]bool{"交換場所なし": false, "交換場所あり": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			handler := newHandler(db, tx)
			userID := testutil.NewUserBuilder(t, tx).Build()
			if hasPlaces {
				testutil.NewUserStationBuilder(t, tx, userID, testutil.NewStationBuilder(t, tx).Build()).Build()
			}

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
			ctx = middleware.SetUserToContext(ctx, &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleJa})
			rec := httptest.NewRecorder()
			handler.Show(rec, req.WithContext(ctx))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			for _, want := range []string{"交換をはじめる準備", `href="/settings/places"`, "まだ入れていません"} {
				if got := strings.Contains(body, want); got == hasPlaces {
					t.Errorf("レスポンスボディに %q が含まれるか = %v、期待値 = %v", want, got, !hasPlaces)
				}
			}
		})
	}
}

// TestShow_Matches は、マッチ候補が1人以上ならブランド色のカードでマッチ候補の画面へリンクし、
// 候補がいないときはマッチ候補の画面へリンクせず、ほしいリストを見直す案内を出すことを検証する。
func TestShow_Matches(t *testing.T) {
	t.Parallel()

	for name, hasMatch := range map[string]bool{"候補なし": false, "候補あり": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			handler := newHandler(db, tx)
			userID := testutil.NewUserBuilder(t, tx).Build()
			if hasMatch {
				categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()
				giveGoodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
				wantGoodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
				stationID := testutil.NewStationBuilder(t, tx).Build()
				partnerID := testutil.NewUserBuilder(t, tx).Build()
				for _, id := range []model.UserID{userID, partnerID} {
					testutil.NewUserStationBuilder(t, tx, id, stationID).Build()
				}
				testutil.NewItemBuilder(t, tx, userID, giveGoodsID).Build()
				testutil.NewItemBuilder(t, tx, userID, wantGoodsID).WithKind(model.ItemKindWant).Build()
				testutil.NewItemBuilder(t, tx, partnerID, wantGoodsID).Build()
				testutil.NewItemBuilder(t, tx, partnerID, giveGoodsID).WithKind(model.ItemKindWant).Build()
			}

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
			ctx = middleware.SetUserToContext(ctx, &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleJa})
			rec := httptest.NewRecorder()
			handler.Show(rec, req.WithContext(ctx))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			matchCard := `<a href="/matches" class="flex items-center gap-3 rounded-lg border border-brand-border bg-brand-subtle p-3">`
			wants := []string{"交換できる相手はまだいません", "ほしいものを増やすと、見つかりやすくなります。", `<a href="/list?kind=want" class="py-1 text-sm font-medium text-primary">リストを見直す</a>`}
			absents := []string{`href="/matches"`, "交換できる相手が"}
			if hasMatch {
				wants = []string{matchCard, "交換できる相手が1人"}
				absents = []string{"交換できる相手はまだいません", "リストを見直す"}
			}
			for _, w := range wants {
				if !strings.Contains(body, w) {
					t.Errorf("レスポンスボディに %q が含まれていない", w)
				}
			}
			for _, a := range absents {
				if strings.Contains(body, a) {
					t.Errorf("レスポンスボディに %q が含まれている", a)
				}
			}
		})
	}
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かのホームとして描画しないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	handler := newHandler(db, tx)
	rec := httptest.NewRecorder()

	handler.Show(rec, httptest.NewRequest(http.MethodGet, "/home", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
