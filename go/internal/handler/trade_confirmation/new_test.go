package trade_confirmation_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/trade_confirmation"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// fixture は、申し込む人と申し込まれる人と、2人の間で交換できるアイテムの1つずつ。
type fixture struct {
	proposer      *model.User
	receiver      *model.User
	receiveItemID model.ItemID
	giveItemID    model.ItemID
}

// newFixture はテストのトランザクションの中に、交換できるアイテムを持つ2人を作る。consent がtrueなら申し込む人を同意済みにする。
func newFixture(t *testing.T, tx *sql.Tx, consent bool) fixture {
	t.Helper()

	categoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).WithName("B賞").Build()
	wantedGoods := testutil.NewGoodsBuilder(t, tx, categoryID).WithName("りすの子").Build()
	offeredGoods := testutil.NewGoodsBuilder(t, tx, categoryID).WithName("くまの子").Build()
	proposer := newUser(t, tx)
	receiver := newUser(t, tx)
	if consent {
		testutil.NewMessageConsentBuilder(t, tx, proposer.ID).Build()
	}
	testutil.NewItemBuilder(t, tx, proposer.ID, wantedGoods).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, tx, receiver.ID, offeredGoods).WithKind(model.ItemKindWant).Build()

	return fixture{
		proposer:      proposer,
		receiver:      receiver,
		receiveItemID: testutil.NewItemBuilder(t, tx, receiver.ID, wantedGoods).Build(),
		giveItemID:    testutil.NewItemBuilder(t, tx, proposer.ID, offeredGoods).Build(),
	}
}

// newUser はテストのトランザクションの中にユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func newUser(t *testing.T, tx *sql.Tx) *model.User {
	t.Helper()

	atname := testutil.UniqueAtname()
	return &model.User{ID: testutil.NewUserBuilder(t, tx).WithAtname(atname).Build(), Atname: atname, Locale: model.LocaleJa}
}

// serve は、ユーザー user としてアットネーム atname の GET /@{atname}/trades/new/confirmation をクエリ query で処理した応答を返す。
func serve(tx *sql.Tx, user *model.User, atname string, query url.Values) *httptest.ResponseRecorder {
	db := testutil.GetTestDB()
	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}
	handler := trade_confirmation.NewHandler(cfg, httperror.NewRenderer(cfg), usecase.NewGetTradeProposalUsecase(
		repository.NewEventCategoryRepository(db).WithTx(tx),
		repository.NewGoodsRepository(db).WithTx(tx),
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewMessageConsentRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	))

	req := httptest.NewRequest(http.MethodGet, "/@"+atname+"/trades/new/confirmation?"+query.Encode(), nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("atname", atname)
	ctx := context.WithValue(i18n.SetLocale(req.Context(), i18n.LangJa), chi.RouteCtxKey, routeCtx)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}
	rec := httptest.NewRecorder()
	handler.New(rec, req.WithContext(ctx))

	return rec
}

// assertContains はボディに want のすべてが含まれることを検証する。
func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// TestNew は、選んだ組み合わせを渡すもの・もらうものに分けて並べ、選んだアイテムを隠して載せたフォームで申し込めるようにし、
// 申し込むと承認の前でもメッセージでやり取りできることの案内と、選んだ状態のまま組み合わせを変えに戻るリンクを出すことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	receiveID, giveID := f.receiveItemID.String(), f.giveItemID.String()

	rec := serve(tx, f.proposer, f.receiver.Atname, url.Values{"receive_item_ids": {receiveID}, "give_item_ids": {giveID}})

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"<title>申し込み内容の確認 | Cutre</title>",
		`<h1 class="text-xl font-semibold">申し込み内容の確認</h1>`,
		`href="/@`+f.receiver.Atname+`" class="block`,
		`action="/trades" method="post"`,
		`name="csrf_token"`,
		`name="atname" value="`+f.receiver.Atname+`"`,
		`name="receive_item_ids" value="`+receiveID+`"`,
		`name="give_item_ids" value="`+giveID+`"`,
		"あなたが渡すもの (1点)",
		"B賞・くまの子",
		"@"+f.receiver.Atname+" さんからもらうもの (1点)",
		"B賞・りすの子",
		"メッセージの1通目として @"+f.receiver.Atname+" さんに届きます",
		`aria-describedby="trade-confirmation-submit-hint">この内容で申し込む</button>`,
		"申し込むと、承認される前でも @"+f.receiver.Atname+" さんとメッセージでやり取りできます。",
		`href="/@`+f.receiver.Atname+`/trades/new?give_item_ids=`+giveID+`&amp;receive_item_ids=`+receiveID+`"`,
	)
}

// TestNew_NotTradable は、もらうものか渡すものを選んでいない (今は交換できないアイテムだけを選んだときを含む) ときは、
// 組み合わせを選ぶ画面を、選んでいない欄のエラー付きで描き直すことを検証する。
func TestNew_NotTradable(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)

	rec := serve(tx, f.proposer, f.receiver.Atname, url.Values{"receive_item_ids": {f.receiveItemID.String()}, "give_item_ids": {f.receiveItemID.String()}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>組み合わせを選ぶ | Cutre</title>",
		"渡すものを1点以上選んでください",
		`name="receive_item_ids" value="`+f.receiveItemID.String()+`" checked>`,
		`name="give_item_ids" value="`+f.giveItemID.String()+`" aria-invalid="true"`,
	)
	if count := strings.Count(body, `aria-invalid="true"`); count != 1 {
		t.Errorf("無効なチェックボックスの数 = %d、渡すものの1つだけを期待", count)
	}
	if strings.Contains(body, "もらうものを1点以上選んでください") {
		t.Error("もらうものを選んでいるのに、もらうもののエラーを出した")
	}
}

// TestNew_MessageConsentRequired は、有効な同意が無いときは、申し込むフォームの代わりにメッセージの利用の画面への案内を出すことを検証する。
func TestNew_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, false)

	rec := serve(tx, f.proposer, f.receiver.Atname, url.Values{"receive_item_ids": {f.receiveItemID.String()}, "give_item_ids": {f.giveItemID.String()}})

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "メッセージの利用への同意が必要です", `href="/settings/message_consent"`)
	if strings.Contains(body, `action="/trades"`) {
		t.Error("同意が無いのに、申し込むフォームを出した")
	}
}

// TestNew_NotFound は、いないユーザーと自分のアットネームを存在しないページとして扱うことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	proposer := newUser(t, tx)

	for name, atname := range map[string]string{"いない": testutil.UniqueAtname(), "自分自身": proposer.Atname} {
		if rec := serve(tx, proposer, atname, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestNew_WithoutUser は、RequireAuth を通さずに届いたリクエストで描画しないことを検証する。
func TestNew_WithoutUser(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	if rec := serve(tx, nil, "yuzu", nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
