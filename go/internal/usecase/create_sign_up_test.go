package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newCreateSignUpUsecase はテスト用のデータベースに直接書き込む CreateSignUpUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newCreateSignUpUsecase(t *testing.T) *usecase.CreateSignUpUsecase {
	t.Helper()

	db := testutil.GetTestDB()
	jobs, err := dispatcher.NewDispatcher(db)
	if err != nil {
		t.Fatalf("NewDispatcher()のエラー = %v", err)
	}

	return usecase.NewCreateSignUpUsecase(
		db,
		repository.NewInvitationRepository(db),
		repository.NewInvitationRedemptionRepository(db),
		validator.NewSignUpCreateValidator(repository.NewUserRepository(db)),
		repository.NewEmailConfirmationRepository(db),
		jobs,
	)
}

// jobKinds は指定したメールアドレス宛てに投入されたジョブの種類と、そのジョブのコードを返す。
func jobKinds(t *testing.T, email string) map[string]string {
	t.Helper()

	rows, err := testutil.GetTestDB().QueryContext(context.Background(),
		"SELECT kind, COALESCE(args->>'code', '') FROM river_job WHERE args->>'email' = $1", email)
	if err != nil {
		t.Fatalf("ジョブの取得のエラー = %v", err)
	}
	defer func() { _ = rows.Close() }()

	kinds := map[string]string{}
	for rows.Next() {
		var kind, code string
		if err := rows.Scan(&kind, &code); err != nil {
			t.Fatalf("ジョブの読み取りのエラー = %v", err)
		}
		kinds[kind] = code
	}

	return kinds
}

// TestCreateSignUpUsecase_Execute は、未登録のアドレスには確認コードのメールを、
// 登録済みのアドレスにはログインを案内するメールを送るジョブを投入し、どちらも確認の行を作ることを検証する。
func TestCreateSignUpUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc := newCreateSignUpUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	invitationID := testutil.NewInvitationBuilder(t, testutil.GetTestDB()).Build()

	registered := testutil.UniqueEmail("create-sign-up-registered")
	testutil.NewUserBuilder(t, testutil.GetTestDB()).WithEmail(registered).Build()

	tests := []struct {
		name     string
		email    string
		wantKind string
	}{
		{name: "未登録のアドレス", email: testutil.UniqueEmail("create-sign-up-new"), wantKind: "send_email_confirmation"},
		{name: "登録済みのアドレス", email: registered, wantKind: "send_already_registered_notice"},
	}

	for _, tt := range tests {
		output, err := uc.Execute(ctx, usecase.CreateSignUpInput{InvitationID: invitationID, Email: tt.email, Locale: model.LocaleEn})
		if err != nil {
			t.Fatalf("%s: Execute()のエラー = %v", tt.name, err)
		}

		confirmation := output.EmailConfirmation
		if confirmation.Email != tt.email || len(confirmation.Code) != 6 {
			t.Errorf("%s: 確認 = %+v、入力のアドレスと6桁のコードを期待", tt.name, confirmation)
		}

		kinds := jobKinds(t, tt.email)
		if len(kinds) != 1 {
			t.Fatalf("%s: ジョブ = %v、1件を期待", tt.name, kinds)
		}
		code, ok := kinds[tt.wantKind]
		if !ok {
			t.Fatalf("%s: ジョブ = %v、%s を期待", tt.name, kinds, tt.wantKind)
		}
		// 確認コードは未登録のアドレスにだけ送る。登録済みのアドレスへのメールには載せない。
		if tt.wantKind == "send_email_confirmation" && code != confirmation.Code {
			t.Errorf("%s: ジョブのコード = %q、確認のコード %q を期待", tt.name, code, confirmation.Code)
		}
		if tt.wantKind == "send_already_registered_notice" && code != "" {
			t.Errorf("%s: 登録済みの案内のジョブにコード %q が載っている", tt.name, code)
		}
	}
}

// TestCreateSignUpUsecase_Execute_Invalid は、形式の誤りでは確認の行もジョブも作らないことを検証する。
func TestCreateSignUpUsecase_Execute_Invalid(t *testing.T) {
	t.Parallel()

	uc := newCreateSignUpUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	invitationID := testutil.NewInvitationBuilder(t, testutil.GetTestDB()).Build()

	const email = "not-an-email"
	_, err := uc.Execute(ctx, usecase.CreateSignUpInput{InvitationID: invitationID, Email: email, Locale: model.LocaleJa})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("email") {
		t.Fatalf("Execute()のエラー = %v、emailのフィールドエラーを期待", err)
	}
	if kinds := jobKinds(t, email); len(kinds) != 0 {
		t.Errorf("ジョブ = %v、投入しないことを期待", kinds)
	}
}

// TestCreateSignUpUsecase_Execute_UnusableInvitation は招待が無いか失効した場合、メールのジョブを投入しないことを検証する。
func TestCreateSignUpUsecase_Execute_UnusableInvitation(t *testing.T) {
	t.Parallel()

	uc := newCreateSignUpUsecase(t)
	db := testutil.GetTestDB()
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	usedAdminID := testutil.NewInvitationBuilder(t, db).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, usedAdminID).Build()
	fullID := inviterInvitationWithRemaining(t, 0)
	tests := []struct {
		name string
		id   model.InvitationID
	}{
		{name: "招待無し"},
		{name: "取り消し済み", id: testutil.NewInvitationBuilder(t, db).WithRevokedAt(time.Now()).Build()},
		{name: "期限切れ", id: testutil.NewInvitationBuilder(t, db).WithExpiresAt(time.Now().Add(-time.Minute)).Build()},
		{name: "使用済みの管理者の招待", id: usedAdminID},
		{name: "招待者の累計人数の上限に達した招待", id: fullID},
	}
	for _, tt := range tests {
		email := testutil.UniqueEmail("create-sign-up-unusable-invitation")
		_, err := uc.Execute(ctx, usecase.CreateSignUpInput{InvitationID: tt.id, Email: email, Locale: model.LocaleJa})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: Execute()のエラー = %v、招待が使えないエラーを期待", tt.name, err)
		}
		var confirmationCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM email_confirmations WHERE email = $1", email).Scan(&confirmationCount); err != nil {
			t.Fatalf("%s: 確認の行の件数の取得のエラー = %v", tt.name, err)
		}
		if confirmationCount != 0 {
			t.Errorf("%s: 確認の行の件数 = %d、期待値 = 0", tt.name, confirmationCount)
		}
		if kinds := jobKinds(t, email); len(kinds) != 0 {
			t.Errorf("%s: ジョブ = %v、投入しないことを期待", tt.name, kinds)
		}
	}
}

// TestCreateSignUpUsecase_Execute_InvitationRevokedDuringRequest は、取り消しとの競合時に確認とメールを作らないことを検証する。
func TestCreateSignUpUsecase_Execute_InvitationRevokedDuringRequest(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	id := testutil.NewInvitationBuilder(t, db).Build()
	email := testutil.UniqueEmail("create-sign-up-race")
	revokeTx, err := db.Begin()
	if err != nil {
		t.Fatalf("取り消し用トランザクションの開始に失敗: %v", err)
	}
	defer func() { _ = revokeTx.Rollback() }()
	if _, err := revokeTx.Exec("UPDATE invitations SET revoked_at = NOW() WHERE id = $1", uuid.UUID(id)); err != nil {
		t.Fatalf("招待の取り消しに失敗: %v", err)
	}

	uc := newCreateSignUpUsecase(t)
	result := make(chan error, 1)
	go func() {
		_, err := uc.Execute(i18n.SetLocale(context.Background(), i18n.LangJa), usecase.CreateSignUpInput{
			InvitationID: id,
			Email:        email,
			Locale:       model.LocaleJa,
		})
		result <- err
	}()

	select {
	case err := <-result:
		t.Fatalf("取り消しの確定前に登録が完了した: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := revokeTx.Commit(); err != nil {
		t.Fatalf("招待の取り消しの確定に失敗: %v", err)
	}

	select {
	case err := <-result:
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("Execute()のエラー = %v、取り消し済みの招待を拒むエラーを期待", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("招待の取り消しの確定後も登録が完了しない")
	}
	if kinds := jobKinds(t, email); len(kinds) != 0 {
		t.Errorf("ジョブ = %v、投入しないことを期待", kinds)
	}
}

// TestCreateSignUpUsecase_Execute_LimitReachedDuringRequest は、登録開始の事前判定後に
// 最後の招待枠が埋まったとき、ロック後の再判定で確認とメールを作らないことを検証する。
func TestCreateSignUpUsecase_Execute_LimitReachedDuringRequest(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	id := inviterInvitationWithRemaining(t, 1)
	email := testutil.UniqueEmail("create-sign-up-limit-race")

	// 招待の行を保持し、登録開始が事前判定を終えてFOR SHAREで待つまで止める。
	lockTx, err := db.Begin()
	if err != nil {
		t.Fatalf("枠を埋めるトランザクションの開始のエラー = %v", err)
	}
	defer func() { _ = lockTx.Rollback() }()
	var lockedID uuid.UUID
	if err := lockTx.QueryRow("SELECT id FROM invitations WHERE id = $1 FOR UPDATE", uuid.UUID(id)).Scan(&lockedID); err != nil {
		t.Fatalf("招待のロックのエラー = %v", err)
	}
	var lockPID int
	if err := lockTx.QueryRow("SELECT pg_backend_pid()").Scan(&lockPID); err != nil {
		t.Fatalf("ロックを保持する接続のPIDの取得のエラー = %v", err)
	}

	uc := newCreateSignUpUsecase(t)
	result := make(chan error, 1)
	go func() {
		_, err := uc.Execute(i18n.SetLocale(context.Background(), i18n.LangJa), usecase.CreateSignUpInput{
			InvitationID: id,
			Email:        email,
			Locale:       model.LocaleJa,
		})
		result <- err
	}()

	// ロック待ちを観測してから枠を埋め、トランザクション内の再判定を確実に通す。
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-result:
			t.Fatalf("招待のロックを解放する前に登録開始が終わった: %v", err)
		default:
		}
		var waiting bool
		err := db.QueryRow(
			`SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
				  AND query LIKE '%FOR SHARE%'
			)`, lockPID,
		).Scan(&waiting)
		if err != nil {
			t.Fatalf("ロック待ちの確認のエラー = %v", err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("登録開始が招待の共有ロックを待たなかった")
		}
		time.Sleep(10 * time.Millisecond)
	}

	testutil.NewInvitationRedemptionBuilder(t, lockTx, id).Build()
	if err := lockTx.Commit(); err != nil {
		t.Fatalf("最後の招待枠の使用の確定のエラー = %v", err)
	}

	select {
	case err := <-result:
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("Execute()のエラー = %v、人数の上限による招待不可のエラーを期待", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("最後の招待枠の使用を確定した後も登録開始が終わらない")
	}

	var confirmationCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM email_confirmations WHERE email = $1", email).Scan(&confirmationCount); err != nil {
		t.Fatalf("確認の行の件数の取得のエラー = %v", err)
	}
	if confirmationCount != 0 {
		t.Errorf("確認の行の件数 = %d、期待値 = 0", confirmationCount)
	}
	if kinds := jobKinds(t, email); len(kinds) != 0 {
		t.Errorf("ジョブ = %v、投入しないことを期待", kinds)
	}
}
