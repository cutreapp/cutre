package list_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/list"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestIndex は、クエリで選んだリストのアイテムをイベントごとの欄に並べ、各行をアイテムの編集の画面へのリンクにし、
// 表示中のリストの切り替えのリンクを押した状態で示すことを検証する。
func TestIndex(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	handler := list.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		usecase.NewGetListUsecase(repository.NewEventRepository(db).WithTx(tx), repository.NewEventCategoryRepository(db).WithTx(tx), repository.NewGoodsRepository(db).WithTx(tx), repository.NewItemRepository(db).WithTx(tx)),
	)
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).WithName("ふわりす もちもちくじ").Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).WithName("B賞").Build()
	giveID := testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).WithName("くまの子").Build()).WithQuantity(2).WithNote("未開封").Build()
	wantID := testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).WithName("ねこの子").Build()).WithKind(model.ItemKindWant).Build()

	tests := []struct {
		name   string
		target string
		want   []string
		absent []string
	}{
		{
			name:   "リストを選ばないときは譲れるリスト",
			target: "/list",
			want: []string{
				`<h1 class="text-2xl font-bold">リスト</h1>`,
				`href="/list?kind=give" class="btn w-full rounded-full" data-variant="ghost" aria-current="page">譲れる <span class="tabular-nums">2</span>`,
				`href="/list?kind=want" class="btn w-full rounded-full" data-variant="ghost">ほしい <span class="tabular-nums">1</span>`,
				"譲れるリストに追加",
				`href="/events"`,
				"ふわりす もちもちくじ",
				`href="/items/` + giveID.String() + `/edit"`,
				"B賞・くまの子",
				"2点・未開封",
				`href="/list" class=`,
			},
			absent: []string{wantID.String()},
		},
		{
			name:   "ほしいリスト",
			target: "/list?kind=want",
			want:   []string{`data-variant="ghost" aria-current="page">ほしい`, "ほしいリストに追加", `href="/items/` + wantID.String() + `/edit"`, "B賞・ねこの子"},
			absent: []string{giveID.String()},
		},
		{
			name:   "読めないリストは譲れるリスト",
			target: "/list?kind=trade",
			want:   []string{`data-variant="ghost" aria-current="page">譲れる`},
		},
	}
	// ケースは1つのテストのトランザクションを使い回すため、サブテストに分けて並行させず、順に確かめる。
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.target, nil)
		ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
		ctx = middleware.SetUserToContext(ctx, &model.User{ID: userID, Atname: "cutre_user"})
		rec := httptest.NewRecorder()
		handler.Index(rec, req.WithContext(ctx))

		if rec.Code != http.StatusOK {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusOK)
			continue
		}
		body := rec.Body.String()
		for _, want := range tt.want {
			if !strings.Contains(body, want) {
				t.Errorf("%s: レスポンスボディに %q が含まれていない", tt.name, want)
			}
		}
		for _, absent := range tt.absent {
			if strings.Contains(body, absent) {
				t.Errorf("%s: レスポンスボディに %q が含まれている", tt.name, absent)
			}
		}
	}
}

// TestIndex_Empty は、アイテムが無いリストに、まだアイテムが無いことを示すことを検証する。
func TestIndex_Empty(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	handler := list.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		usecase.NewGetListUsecase(repository.NewEventRepository(db).WithTx(tx), repository.NewEventCategoryRepository(db).WithTx(tx), repository.NewGoodsRepository(db).WithTx(tx), repository.NewItemRepository(db).WithTx(tx)),
	)
	req := httptest.NewRequest(http.MethodGet, "/list?kind=want", nil)
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, &model.User{ID: testutil.NewUserBuilder(t, tx).Build(), Atname: "cutre_user"})
	rec := httptest.NewRecorder()
	handler.Index(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ほしいリストにはまだアイテムがありません") {
		t.Errorf("応答 = %d、アイテムが無いことを示す200を期待", rec.Code)
	}
}

// TestIndex_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かのリストとして描画しないことを検証する。
func TestIndex_WithoutUser(t *testing.T) {
	t.Parallel()

	handler := list.NewHandler(&config.Config{Env: "dev", Domain: "cutre.example.com"}, nil)
	rec := httptest.NewRecorder()
	handler.Index(rec, httptest.NewRequest(http.MethodGet, "/list", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
