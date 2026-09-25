package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
)

var (
	_ Sender = (*ResendSender)(nil)
	_ Sender = (*LogSender)(nil)
)

// testInput は送信の検証に使う1通のメール。
func testInput() SendInput {
	return SendInput{
		To:       "user@example.com",
		Subject:  "確認コード",
		HTMLBody: templ.Raw("<p>123456</p>"),
		TextBody: templ.Raw("123456"),
	}
}

// newTestResendSender はhandlerが応答するResendのAPIへ送るResendSenderを作る。
//
// URLは本番と同じままにし、通信経路だけをメモリ内のテスト用サーバーへ向ける。
// handlerはメソッド・ホスト・パスまで含めて登録するため、違う宛先へ送ればテストが落ちる。
func newTestResendSender(t *testing.T, handler http.HandlerFunc) *ResendSender {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST api.resend.com/emails", handler)
	server := httptest.NewTestServer(t, mux)

	sender := NewResendSender("re_test", "noreply@cutre.example.com")
	// Timeoutを保つため、http.Client全体ではなく通信経路だけを差し替える。
	sender.httpClient.Transport = server.Client().Transport
	return sender
}

// TestResendSender_Send は、描画した本文と送信元・宛先・件名をAPIキー付きでResendへ送ることを検証する。
func TestResendSender_Send(t *testing.T) {
	t.Parallel()

	var got map[string]any
	var authorization, idempotencyKey string
	sender := newTestResendSender(t, func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			t.Error("HTTPSで送られていない")
		}
		authorization = r.Header.Get("Authorization")
		idempotencyKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("リクエストボディのデコードのエラー = %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"test-email-id"}`))
	})

	input := testInput()
	input.IdempotencyKey = "confirmation-123"
	if err := sender.Send(context.Background(), input); err != nil {
		t.Fatalf("Send()のエラー = %v", err)
	}

	if idempotencyKey != input.IdempotencyKey {
		t.Errorf("Idempotency-Key = %q、期待値 = %q", idempotencyKey, input.IdempotencyKey)
	}
	if authorization != "Bearer re_test" {
		t.Errorf("Authorization = %q、期待値 = %q", authorization, "Bearer re_test")
	}
	wants := map[string]any{
		"from":    "Cutre <noreply@cutre.example.com>",
		"to":      []any{"user@example.com"},
		"subject": "確認コード",
		"html":    "<p>123456</p>",
		"text":    "123456",
	}
	for key, want := range wants {
		gotJSON, _ := json.Marshal(got[key])
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s = %s、期待値 = %s", key, gotJSON, wantJSON)
		}
	}
}

// TestResendSender_Send_Error は、Resendが送信を拒んだときにエラーを返すことを検証する。
// ワーカーはこのエラーを見てRiverに再試行させる。
func TestResendSender_Send_Error(t *testing.T) {
	t.Parallel()

	sender := newTestResendSender(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"statusCode":422,"name":"validation_error","message":"invalid"}`))
	})

	if err := sender.Send(context.Background(), testInput()); err == nil {
		t.Error("エラーを期待したが、nilだった")
	}
}

// TestLogSender_Send は、メールの宛先・件名・テキスト本文がログへ出ることを検証する。
func TestLogSender_Send(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	if err := NewLogSender().Send(context.Background(), testInput()); err != nil {
		t.Fatalf("Send()のエラー = %v", err)
	}

	var got map[string]any
	if err := json.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("ログのデコードのエラー = %v", err)
	}
	for key, want := range map[string]string{
		"to":      "user@example.com",
		"subject": "確認コード",
		"body":    "123456",
	} {
		if got[key] != want {
			t.Errorf("ログの%s = %v、期待値 = %q", key, got[key], want)
		}
	}
}

// TestSend_RenderError は、本文の描画に失敗したメールを送らずにエラーを返すことを検証する。
func TestSend_RenderError(t *testing.T) {
	t.Parallel()

	renderErr := errors.New("描画の失敗")
	broken := templ.ComponentFunc(func(context.Context, io.Writer) error { return renderErr })

	requested := false
	resendSender := newTestResendSender(t, func(http.ResponseWriter, *http.Request) { requested = true })

	for name, sender := range map[string]Sender{"ResendSender": resendSender, "LogSender": NewLogSender()} {
		for _, input := range []SendInput{
			{To: "user@example.com", HTMLBody: broken, TextBody: templ.Raw("")},
			{To: "user@example.com", HTMLBody: templ.Raw(""), TextBody: broken},
		} {
			if err := sender.Send(context.Background(), input); !errors.Is(err, renderErr) {
				t.Errorf("%sのSend()のエラー = %v、描画のエラーを期待", name, err)
			}
		}
	}
	if requested {
		t.Error("描画に失敗したメールをResendへ送った")
	}
}
