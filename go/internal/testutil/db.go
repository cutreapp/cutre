// Package testutil はテスト用のヘルパーを提供する。
// テスト用データベースへの接続と、そこへ行を投入するビルダーを持つ。
package testutil

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/database"
)

// testDB はパッケージ内の全テストで共有するDB接続プール。
var (
	testDB     *sql.DB
	testDBErr  error
	testDBOnce sync.Once
)

// initTestDB はテスト用DBへの接続を1度だけ確立する。
//
// あわせてbcryptのコストを下げるのも、それがパッケージレベルの変数であり、
// 並行するテストがそれぞれ書き込むと競合するため。
// 初期化は SetupTestMain / SetupTx / GetTestDB のいずれから呼ばれても同じ接続を返す。
func initTestDB() (*sql.DB, error) {
	testDBOnce.Do(func() {
		auth.BcryptCost = auth.TestBcryptCost

		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			testDBErr = fmt.Errorf("テスト用の環境変数 DATABASE_URL が設定されていません")
			return
		}

		testDB, testDBErr = database.Connect(dsn)
	})

	return testDB, testDBErr
}

// SetupTestMain は TestMain 内で呼び出し、パッケージ共有のDB接続を先に初期化する。
// 戻り値を os.Exit に渡す。
// SetupTx / GetTestDB が初回呼び出しで初期化するため、呼び出しは任意。
//
// 使用例:
//
//	func TestMain(m *testing.M) {
//	    os.Exit(testutil.SetupTestMain(m))
//	}
func SetupTestMain(m *testing.M) int {
	if _, err := initTestDB(); err != nil {
		slog.Error("テスト用データベースの初期化に失敗しました", "error", err)
		return 1
	}

	return m.Run()
}

// GetTestDB は共有のDB接続プールを返す。
// UseCaseのテストのように、自前でトランザクションを管理するテストで使う。
// 初期化に失敗した場合はpanicする。接続できないのは個々のテストの失敗ではなく、
// テストを走らせる環境が整っていないことを意味するため。
func GetTestDB() *sql.DB {
	db, err := initTestDB()
	if err != nil {
		panic(fmt.Sprintf("テスト用データベースの初期化に失敗しました: %v", err))
	}

	return db
}

// SetupTx はテスト用のトランザクションを開始し、テストの終了時にロールバックする。
// テストが書いた行がロールバックで消えるため、テストどうしがデータで干渉しない。
func SetupTx(t *testing.T) (*sql.DB, *sql.Tx) {
	t.Helper()

	db := GetTestDB()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("トランザクションの開始に失敗しました: %v", err)
	}

	t.Cleanup(func() {
		_ = tx.Rollback()
	})

	return db, tx
}
