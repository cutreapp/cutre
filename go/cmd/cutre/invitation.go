package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/database"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// runInvitation は `cutre invitation <操作>` を振り分け、プロセスの終了コードを返す。
func runInvitation(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "create" {
		// 書き込みエラーを捨てる理由はrunと同じ。
		_, _ = fmt.Fprint(stderr, `使い方: cutre invitation <操作>

操作:
  create     招待者の無い招待リンクを発行する
`)

		return exitUsage
	}

	return runInvitationCreate(stdout)
}

// runInvitationCreate は招待者の無い招待リンクを発行し、そのURLを標準出力へ書く。
//
// 最初のユーザーの登録と運営からの招待に使うため、本番を含むどの環境でも実行できる。
// 標準出力にはURLだけを書き、スクリプトから受け取れるようにする。有効期限などはログ (標準エラー出力) に出す。
func runInvitationCreate(stdout io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定の読み込みに失敗しました", "error", err)
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

	uc := usecase.NewCreateInvitationUsecase(repository.NewInvitationRepository(db))
	output, err := uc.Execute(context.Background())
	if err != nil {
		slog.Error("招待の発行に失敗しました", "error", err)
		return 1
	}

	invitation := output.Invitation
	slog.Info("招待リンクを発行しました", "invitation_id", invitation.ID.String(), "expires_at", invitation.ExpiresAt)

	if _, err := fmt.Fprintln(stdout, cfg.AppURL()+templates.InvitationPath(invitation.Token)); err != nil {
		slog.Error("招待リンクの出力に失敗しました", "error", err)
		return 1
	}

	return 0
}
