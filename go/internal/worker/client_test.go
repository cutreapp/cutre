package worker_test

import (
	"context"
	"os"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/worker"
)

// TestNewClient は、ワーカーと定期ジョブを登録したクライアントを組み立てられることを検証する。
//
// Riverは登録の誤り (同じ種類のワーカーの重複・ワーカーの無い定期ジョブなど) を組み立ての時点で拒む。
// 起動はしない。起動すると定期ジョブがすぐに走り、テスト用データベースを共有する他のパッケージの行を消しうるため。
func TestNewClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := &config.Config{DatabaseURL: os.Getenv("DATABASE_URL")}

	client, err := worker.NewClient(ctx, cfg, testutil.GetTestDB())
	if err != nil {
		t.Fatalf("NewClient()のエラー = %v", err)
	}
	t.Cleanup(func() {
		if err := client.Stop(ctx); err != nil {
			t.Errorf("Stop()のエラー = %v", err)
		}
	})

	if client.Client() == nil {
		t.Error("Client() = nil、基盤のRiverのクライアントを期待")
	}
}

// TestNewClient_InvalidDatabaseURL は、接続文字列として読めない値を渡すとエラーを返すことを検証する。
func TestNewClient_InvalidDatabaseURL(t *testing.T) {
	t.Parallel()

	client, err := worker.NewClient(context.Background(), &config.Config{DatabaseURL: "postgres://%zz"}, testutil.GetTestDB())
	if err == nil {
		t.Fatal("エラーを期待したが、nilだった")
	}
	if client != nil {
		t.Errorf("client = %v、期待値 = nil", client)
	}
}
