package dispatcher_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestDispatcher_Enqueue は、各ジョブを種類・引数・試行回数の上限付きで、渡したトランザクションの中に投入することを検証する。
func TestDispatcher_Enqueue(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()

	d, err := dispatcher.NewDispatcher(db)
	if err != nil {
		t.Fatalf("NewDispatcher()のエラー = %v", err)
	}
	d = d.WithTx(tx)

	email := testutil.UniqueEmail("dispatcher")
	if err := d.EnqueueEmailConfirmation(ctx, email, "012345", model.LocaleEn); err != nil {
		t.Fatalf("EnqueueEmailConfirmation()のエラー = %v", err)
	}
	if err := d.EnqueueAlreadyRegisteredNotice(ctx, email, model.LocaleJa); err != nil {
		t.Fatalf("EnqueueAlreadyRegisteredNotice()のエラー = %v", err)
	}

	tests := []struct {
		kind     string
		wantArgs map[string]string
	}{
		{kind: "send_email_confirmation", wantArgs: map[string]string{"email": email, "code": "012345", "locale": "en"}},
		{kind: "send_already_registered_notice", wantArgs: map[string]string{"email": email, "locale": "ja"}},
	}
	for _, tt := range tests {
		var rawArgs []byte
		var maxAttempts int
		if err := tx.QueryRowContext(ctx,
			"SELECT args, max_attempts FROM river_job WHERE kind = $1 AND args->>'email' = $2", tt.kind, email,
		).Scan(&rawArgs, &maxAttempts); err != nil {
			t.Fatalf("%sのジョブの取得のエラー = %v", tt.kind, err)
		}

		var args map[string]string
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			t.Fatalf("%sの引数のデコードのエラー = %v", tt.kind, err)
		}
		for key, want := range tt.wantArgs {
			if args[key] != want {
				t.Errorf("%sの引数 %s = %q、期待値 = %q", tt.kind, key, args[key], want)
			}
		}
		if maxAttempts != 5 {
			t.Errorf("%sのmax_attempts = %d、期待値 = 5", tt.kind, maxAttempts)
		}
	}
}

// TestDispatcher_EnqueuePasswordReset は、パスワードリセットのジョブをユーザーのIDと言語だけを引数にして投入することを検証する。
func TestDispatcher_EnqueuePasswordReset(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()

	d, err := dispatcher.NewDispatcher(db)
	if err != nil {
		t.Fatalf("NewDispatcher()のエラー = %v", err)
	}

	userID := testutil.NewUserBuilder(t, tx).Build()
	if err := d.WithTx(tx).EnqueuePasswordReset(ctx, userID, model.LocaleEn); err != nil {
		t.Fatalf("EnqueuePasswordReset()のエラー = %v", err)
	}

	var rawArgs []byte
	var maxAttempts int
	if err := tx.QueryRowContext(ctx,
		"SELECT args, max_attempts FROM river_job WHERE kind = 'send_password_reset' AND args->>'user_id' = $1", userID.String(),
	).Scan(&rawArgs, &maxAttempts); err != nil {
		t.Fatalf("ジョブの取得のエラー = %v", err)
	}

	var args map[string]string
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		t.Fatalf("引数のデコードのエラー = %v", err)
	}
	want := map[string]string{"user_id": userID.String(), "locale": "en"}
	if len(args) != len(want) || args["user_id"] != want["user_id"] || args["locale"] != want["locale"] {
		t.Errorf("引数 = %v、期待値 = %v (トークンやメールアドレスを載せない)", args, want)
	}
	if maxAttempts != 5 {
		t.Errorf("max_attempts = %d、期待値 = 5", maxAttempts)
	}
}
