package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_Usage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantContains []string
	}{
		{
			name:         "引数無しではusageを表示する",
			args:         []string{},
			wantContains: []string{"使い方: cutre <コマンド>", "serve"},
		},
		{
			name:         "未知のサブコマンドではその名前とusageを表示する",
			args:         []string{"nosuchcommand"},
			wantContains: []string{`未知のサブコマンドです: "nosuchcommand"`, "使い方: cutre <コマンド>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			code := run(tt.args, &stderr)

			if code != exitUsage {
				t.Errorf("終了コード = %d、期待値 = %d", code, exitUsage)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("標準エラー出力 = %q、%qを含むことを期待", stderr.String(), want)
				}
			}
		})
	}
}

// serveが起動に成功するとポートを占有してシャットダウンまでブロックするため、
// 設定の読み込みに失敗させて、振り分けがserveに届き終了コード1が返ることだけを検証する。
// t.Setenv を使うため t.Parallel() は呼ばない。
func TestRun_Serve(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var stderr bytes.Buffer

	code := run([]string{"serve"}, &stderr)

	if code != 1 {
		t.Errorf("終了コード = %d、期待値 = %d", code, 1)
	}
	if strings.Contains(stderr.String(), "使い方:") {
		t.Errorf("標準エラー出力 = %q、usageを含まないことを期待", stderr.String())
	}
}

// seedの振り分けも同じく、設定の読み込みに失敗させて終了コード1が返ることだけを検証する。
// 開発環境以外での拒否は TestRunSeed_RejectsNonDev、名簿の読み込みは internal/seed のテストで確かめる。
// t.Setenv を使うため t.Parallel() は呼ばない。
func TestRun_Seed(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var stderr bytes.Buffer

	if code := run([]string{"seed"}, &stderr); code != 1 {
		t.Errorf("終了コード = %d、期待値 = %d", code, 1)
	}
	if strings.Contains(stderr.String(), "使い方:") {
		t.Errorf("標準エラー出力 = %q、usageを含まないことを期待", stderr.String())
	}
}
