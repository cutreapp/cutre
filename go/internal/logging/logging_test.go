package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/logging"
)

func TestParseFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    logging.Format
		wantErr bool
	}{
		{name: "空ならText", value: "", want: logging.FormatText},
		{name: "text", value: "text", want: logging.FormatText},
		{name: "json", value: "json", want: logging.FormatJSON},
		{name: "大文字は受け付けない", value: "JSON", wantErr: true},
		{name: "未知の形式", value: "logfmt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := logging.ParseFormat(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("エラーを期待したが、nilだった")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFormat()のエラー = %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseFormat() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestNewBaseHandler_JSON は、JSON形式で値の型を保ったまま出力し、contextのリクエストIDを加えることを検証する。
func TestNewBaseHandler_JSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(logging.NewBaseHandler(&buf, logging.FormatJSON)).With("component", "test")

	ctx := logging.WithRequestID(context.Background(), "REQ123")
	logger.InfoContext(ctx, "処理しました", "status", 200)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("ログのデコードのエラー = %v、出力 = %q", err, buf.String())
	}

	if got := record["msg"]; got != "処理しました" {
		t.Errorf("msg = %v、期待値 = 処理しました", got)
	}
	if got := record["status"]; got != float64(200) {
		t.Errorf("status = %#v、期待値 = 数値の200", got)
	}
	if got := record["request_id"]; got != "REQ123" {
		t.Errorf("request_id = %v、期待値 = REQ123", got)
	}
	if got := record["component"]; got != "test" {
		t.Errorf("component = %v、期待値 = test", got)
	}
}

// TestNewBaseHandler_Text は、Text形式で出力し、リクエストIDが無いcontextでは request_id を出さないことを検証する。
func TestNewBaseHandler_Text(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(logging.NewBaseHandler(&buf, logging.FormatText))

	logger.InfoContext(context.Background(), "処理しました", "status", 200)

	out := buf.String()
	if !strings.Contains(out, "msg=処理しました") || !strings.Contains(out, "status=200") {
		t.Errorf("Text形式の出力になっていない: %q", out)
	}
	if strings.Contains(out, "request_id") {
		t.Errorf("リクエストIDが無いのに request_id が出力された: %q", out)
	}
}

// TestNewBaseHandler_Level は、Debugを出さずInfo以上を出すことを検証する。
func TestNewBaseHandler_Level(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(logging.NewBaseHandler(&buf, logging.FormatText))

	logger.Debug("デバッグ")
	if buf.Len() != 0 {
		t.Errorf("Debugが出力された: %q", buf.String())
	}

	logger.Info("情報")
	if buf.Len() == 0 {
		t.Error("Infoが出力されなかった")
	}
}

// TestNewRequestID は、採番するIDが毎回異なることを検証する。
func TestNewRequestID(t *testing.T) {
	t.Parallel()

	a, b := logging.NewRequestID(), logging.NewRequestID()
	if a == "" || a == b {
		t.Errorf("NewRequestID() = %q, %q、期待値 = 空でない互いに異なる値", a, b)
	}
}

func TestRequestIDFromContext(t *testing.T) {
	t.Parallel()

	if got := logging.RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("IDを載せていないcontextの RequestIDFromContext() = %q、期待値 = 空文字列", got)
	}

	ctx := logging.WithRequestID(context.Background(), "REQ123")
	if got := logging.RequestIDFromContext(ctx); got != "REQ123" {
		t.Errorf("RequestIDFromContext() = %q、期待値 = %q", got, "REQ123")
	}
}
