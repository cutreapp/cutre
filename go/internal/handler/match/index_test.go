package match_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/match"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newHandler はテストのトランザクションで読む Handler を組み立てる。
func newHandler(db *sql.DB, tx *sql.Tx) *match.Handler {
	return match.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		usecase.NewGetMatchesUsecase(
			repository.NewEventCategoryRepository(db).WithTx(tx),
			repository.NewGoodsRepository(db).WithTx(tx),
			repository.NewItemRepository(db).WithTx(tx),
			repository.NewStationRepository(db).WithTx(tx),
			repository.NewUserRepository(db).WithTx(tx),
			repository.NewUserStationRepository(db).WithTx(tx),
		),
	)
}

// serve はユーザー userID として GET /matches を処理した応答を返す。
func serve(handler *match.Handler, userID model.UserID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/matches", nil)
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, &model.User{ID: userID, Atname: "cutre_user", Locale: model.LocaleJa})
	rec := httptest.NewRecorder()
	handler.Index(rec, req.WithContext(ctx))
	return rec
}

// TestIndex は、マッチ候補ごとにプロフィールへのリンク・交換場所・もらえるもの・渡せるものを出し、
// 交換の中の画面としてパンくずとメインメニューを出すことを検証する。
func TestIndex(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	handler := newHandler(db, tx)
	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).WithName("B賞").Build()
	wantedGoods := testutil.NewGoodsBuilder(t, tx, categoryID).WithName("りすの子").Build()
	offeredGoods := testutil.NewGoodsBuilder(t, tx, categoryID).WithName("くまの子").Build()
	shibuya := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).WithName("渋谷").Build()

	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserStationBuilder(t, tx, userID, shibuya).Build()
	testutil.NewItemBuilder(t, tx, userID, wantedGoods).WithKind(model.ItemKindWant).WithQuantity(2).Build()
	testutil.NewItemBuilder(t, tx, userID, offeredGoods).Build()

	partnerAtname := testutil.UniqueAtname()
	partnerID := testutil.NewUserBuilder(t, tx).WithAtname(partnerAtname).Build()
	testutil.NewUserStationBuilder(t, tx, partnerID, shibuya).Build()
	testutil.NewItemBuilder(t, tx, partnerID, wantedGoods).WithQuantity(3).Build()
	testutil.NewItemBuilder(t, tx, partnerID, offeredGoods).WithKind(model.ItemKindWant).Build()

	rec := serve(handler, userID)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	// 交換から辿った画面のため、メインメニューの交換の項目を選択中 (aria-current="true") にする。
	if !regexp.MustCompile(`href="/trades" class="[^"]*" aria-current="true"`).MatchString(body) {
		t.Error("メインメニューの交換の項目が選択中になっていない")
	}
	for _, want := range []string{
		"<title>マッチ候補 | Cutre</title>",
		`<h1 class="text-xl font-semibold">マッチ候補</h1>`,
		`href="/trades" class="block`,
		`href="/@` + partnerAtname + `"`,
		"@" + partnerAtname,
		"<span>東京都 渋谷</span>",
		"もらえる 2点",
		"B賞・りすの子",
		"×2",
		"渡せる 1点",
		"B賞・くまの子",
		`href="/@` + partnerAtname + `/trades/new"`,
		`aria-label="@` + partnerAtname + ` さんに交換を申し込む"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// TestIndex_Empty は、候補がいないときに、交換場所をまだ選んでいなければ交換場所の画面へ、選んでいればリストへ案内することを検証する。
func TestIndex_Empty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hasPlaces bool
		want      []string
	}{
		{name: "交換場所なし", hasPlaces: false, want: []string{"交換場所を入れると、交換できる相手を探しはじめます。", `href="/settings/places"`}},
		{name: "交換場所あり", hasPlaces: true, want: []string{"交換できる相手はまだいません。", `href="/list" class="btn"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			handler := newHandler(db, tx)
			userID := testutil.NewUserBuilder(t, tx).Build()
			if tt.hasPlaces {
				testutil.NewUserStationBuilder(t, tx, userID, testutil.NewStationBuilder(t, tx).Build()).Build()
			}

			rec := serve(handler, userID)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			for _, want := range tt.want {
				if !strings.Contains(rec.Body.String(), want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestIndex_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かのマッチ候補として描画しないことを検証する。
func TestIndex_WithoutUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	rec := httptest.NewRecorder()
	newHandler(db, tx).Index(rec, httptest.NewRequest(http.MethodGet, "/matches", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
