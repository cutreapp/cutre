package settings_invitation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/settings_invitation"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newHandler はテスト用のデータベースに直接書き込む Handler を返す。
// 招待を用意するUseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newHandler() *settings_invitation.Handler {
	db := testutil.GetTestDB()
	invitationRepo := repository.NewInvitationRepository(db)
	invitationRedemptionRepo := repository.NewInvitationRedemptionRepository(db)

	cfg := &config.Config{Env: "dev", Domain: "cutre.example.com"}

	return settings_invitation.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		session.NewFlashManager(),
		usecase.NewPrepareInvitationUsecase(db, repository.NewUserRepository(db), invitationRepo, invitationRedemptionRepo),
		usecase.NewRecreateInvitationUsecase(db, repository.NewUserRepository(db), invitationRepo, invitationRedemptionRepo),
		usecase.NewGetInvitationRedemptionsUsecase(invitationRedemptionRepo),
	)
}

// signedInUser はテスト用のユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func signedInUser(t *testing.T, locale model.Locale) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	id := testutil.NewUserBuilder(t, db).WithLocale(locale).Build()
	user, err := repository.NewUserRepository(db).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// show はログイン中のユーザーとして招待の画面を開く。ロケールは本番では UserLocale が users.locale から決める。
func show(user *model.User) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/settings/invitation", nil)
	ctx := i18n.SetLocale(req.Context(), string(user.Locale))
	ctx = middleware.SetUserToContext(ctx, user)
	rec := httptest.NewRecorder()
	newHandler().Show(rec, req.WithContext(ctx))

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

// assertNotContains はボディに absent のいずれも含まれないことを検証する。
func assertNotContains(t *testing.T, body string, absents ...string) {
	t.Helper()

	for _, absent := range absents {
		if strings.Contains(body, absent) {
			t.Errorf("レスポンスボディに %q が含まれている", absent)
		}
	}
}

// TestShow は、招待を持たないユーザーが開くと招待を作り、そのリンクをQRコード・URL・有効期限と一緒に描画し、
// コピーと共有のボタンを対応するブラウザでだけ出せるよう隠して置くことを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	rec := show(user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	invitation, err := repository.NewInvitationRepository(testutil.GetTestDB()).FindUnrevokedByInviterUserID(context.Background(), user.ID)
	if err != nil || invitation == nil {
		t.Fatalf("作られた招待 = (%+v, %v)、招待を期待", invitation, err)
	}
	tokyo, _ := time.LoadLocation(model.DefaultTimeZone)
	lastDate := invitation.LastUsableDate(tokyo)
	expiresOn := strconv.Itoa(int(lastDate.Month())) + "月" + strconv.Itoa(lastDate.Day()) + "日"
	if lastDate.Year() != time.Now().In(tokyo).Year() {
		expiresOn = strconv.Itoa(lastDate.Year()) + "年" + expiresOn
	}
	url := "https://cutre.example.com/i/" + invitation.Token

	body := rec.Body.String()
	assertContains(t, body,
		"<title>招待 | Cutre</title>",
		`<meta name="robots" content="noindex">`,
		`href="/@`+user.Atname+`"`,
		"実際に会ったことがある人だけを招待してください",
		"QRコードを読み取ると招待の受け取り画面が開きます",
		"あと5人招待できます",
		`role="img" aria-label="招待URLのQRコード"`,
		`class="fill-black" d="M0 0h7v1h-7z`,
		`value="`+url+`"`,
		`data-copy-text="`+url+`"`,
		`data-share-url="`+url+`"`,
		"有効期限 "+expiresOn+"まで",
		"作り直すと、いまのURLとQRコードは使えなくなります",
		`commandfor="invitation-recreate-dialog" command="show-modal"`,
		`<dialog id="invitation-recreate-dialog" class="alert-dialog"`,
		"招待リンクを作り直しますか?",
		`commandfor="invitation-recreate-dialog" command="close"`,
		`<form action="/settings/invitation" method="post"`,
		`name="csrf_token"`,
		`name="invitation_id" value="`+invitation.ID.String()+`"`,
		"まだ参加した人はいません",
	)
	if got := strings.Count(body, "hidden>"); got < 2 {
		t.Errorf("hidden のボタンの数 = %d、コピーと共有の2つ以上を期待", got)
	}
}

// TestShow_Redemptions は、参加した人を新しい順に、退会した人は退会したユーザーとして、参加した日と一緒に描画することを検証する。
// 今年の日付は年を省き、前の年の日付には年を付ける。
func TestShow_Redemptions(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleEn)
	invitationID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()

	tokyo, _ := time.LoadLocation(model.DefaultTimeZone)
	thisYear := time.Now().In(tokyo).Year()
	memberAtname := testutil.UniqueAtname()
	memberID := testutil.NewUserBuilder(t, db).WithAtname(memberAtname).Build()
	withdrawnID := testutil.NewUserBuilder(t, db).WithDeletedAt(time.Now()).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, invitationID).WithUserID(withdrawnID).WithCreatedAt(time.Date(thisYear-1, 12, 31, 20, 0, 0, 0, tokyo)).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, invitationID).WithUserID(memberID).WithCreatedAt(time.Date(thisYear, 1, 2, 0, 30, 0, 0, tokyo)).Build()

	rec := show(user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"Invites left: 3",
		"Scan the QR code to view the invitation",
		"@"+memberAtname,
		"Joined Jan 2<",
		"A withdrawn user",
		"Joined Dec 31, "+strconv.Itoa(thisYear-1),
	)
	if strings.Index(body, "@"+memberAtname) > strings.Index(body, "A withdrawn user") {
		t.Error("参加した人が新しい順に並んでいない")
	}
	assertNotContains(t, body, "No one has joined yet")
}

// TestShow_Full は、招待できる人数の上限に達していれば、招待リンクの代わりに上限に達した旨を描画することを検証する。
func TestShow_Full(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	invitationID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()
	for range model.InviterRedemptionLimit {
		testutil.NewInvitationRedemptionBuilder(t, db, invitationID).Build()
	}

	rec := show(user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "招待できる人数の上限に達しました", "あと0人招待できます", "参加した人")
	assertNotContains(t, body, "招待URLのQRコード", "data-copy-text", "data-share-url", "/i/", "invitation-recreate-dialog")
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストで招待を作らず、描画もしないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/settings/invitation", nil)
	rec := httptest.NewRecorder()
	newHandler().Show(rec, req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
