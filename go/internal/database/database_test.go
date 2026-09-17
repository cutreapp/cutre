package database

import (
	"os"
	"strings"
	"testing"
)

func TestConnect(t *testing.T) {
	t.Parallel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL が設定されていません。make test で実行してください")
	}

	db, err := Connect(dsn)
	if err != nil {
		t.Fatalf("Connectのエラー = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Closeのエラー = %v", err)
		}
	})

	var got int
	if err := db.QueryRow("SELECT 1").Scan(&got); err != nil {
		t.Fatalf("クエリの実行のエラー = %v", err)
	}
	if got != 1 {
		t.Errorf("SELECT 1 の結果 = %d、期待値 = %d", got, 1)
	}
}

func TestConnect_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dsn         string
		wantContain string
	}{
		{
			name:        "DSNとして解釈できない文字列ではエラーを返す",
			dsn:         "postgres://%zz",
			wantContain: "データベースへのPingに失敗しました",
		},
		{
			// ポート1は待ち受けているプロセスが無く、即座に接続を拒否される。
			name:        "接続できないホストではエラーを返す",
			dsn:         "postgres://postgres@127.0.0.1:1/cutre_test?sslmode=disable&connect_timeout=1",
			wantContain: "データベースへのPingに失敗しました",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, err := Connect(tt.dsn)

			if err == nil {
				t.Fatal("エラーが返ることを期待したが、nilだった")
			}
			if db != nil {
				t.Errorf("*sql.DB = %v、期待値 = nil", db)
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("エラー = %q、%qを含むことを期待", err.Error(), tt.wantContain)
			}
		})
	}
}
