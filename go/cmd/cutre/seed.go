package main

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/database"
	"github.com/cutreapp/cutre/go/internal/seed"
)

// runSeed は名簿 (go/seed-users.toml) のアカウントと、マスタ (イベント・カテゴリー・グッズ・駅) の見本を
// 開発用データベースへ作る。戻り値はプロセスの終了コード。
//
// 開発環境以外では実行を拒む。名簿のパスワードは全アカウント共通の単純な値のため、
// 誤って本番へ流すと誰でもログインできるアカウントができてしまう。
func runSeed(stdout io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定の読み込みに失敗しました", "error", err)
		return 1
	}
	if !cfg.IsDev() {
		slog.Error("seed は開発環境でしか実行できません", "env", cfg.Env)
		return 1
	}

	roster, err := seed.LoadRoster(seed.RosterPath)
	if err != nil {
		slog.Error("名簿の読み込みに失敗しました。seed-users.example.toml を seed-users.toml へ複製して書き換えてください", "error", err)
		return 1
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("データベース接続に失敗しました", "error", err)
		return 1
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("データベース接続のクローズに失敗しました", "error", err)
		}
	}()

	ctx := context.Background()
	if err := seed.CreateUsers(ctx, db, roster, stdout); err != nil {
		slog.Error("アカウントの作成に失敗しました", "error", err)
		return 1
	}
	if err := seed.CreateMasters(ctx, db, seed.DefaultMasters(time.Now()), stdout); err != nil {
		slog.Error("マスタの見本の作成に失敗しました", "error", err)
		return 1
	}

	return 0
}
