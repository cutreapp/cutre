package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/database"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// runUser は `cutre user <操作>` を振り分け、プロセスの終了コードを返す。
func runUser(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || args[0] != "disable-two-factor" {
		// 書き込みエラーを捨てる理由はrunと同じ。
		_, _ = fmt.Fprint(stderr, `使い方: cutre user <操作>

操作:
  disable-two-factor <メールアドレスまたは@アットネーム>
             ユーザーの二要素認証を無効にする
`)

		return exitUsage
	}

	return runUserDisableTwoFactor(args[1], stdout)
}

// runUserDisableTwoFactor はユーザーの二要素認証を無効にし、結果を標準出力へ書く。
//
// 認証アプリとリカバリーコードを両方失った人の救済に使うため、本番を含むどの環境でも実行できる。
// 本人確認は実行する管理者が済ませる。ユーザーが見つからないときは失敗として終了コード1を返す。
// 有効にしていなかったときは、何も変わらなかったことを伝えて成功とする。
func runUserDisableTwoFactor(identifier string, stdout io.Writer) int {
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

	uc := usecase.NewForceDisableTwoFactorAuthUsecase(
		db,
		repository.NewUserRepository(db),
		repository.NewUserTwoFactorAuthRepository(db),
		repository.NewUserTwoFactorRecoveryCodeRepository(db),
	)
	output, err := uc.Execute(context.Background(), usecase.ForceDisableTwoFactorAuthInput{Identifier: identifier})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			slog.Error("ユーザーが見つかりません", "identifier", identifier)
			return 1
		}
		slog.Error("二要素認証の無効化に失敗しました", "error", err)
		return 1
	}

	result := "の二要素認証を無効にしました"
	if output.Disabled {
		slog.Info("管理者が二要素認証を無効にしました", "user_id", output.User.ID.String())
	} else {
		result = "は二要素認証を有効にしていません"
	}
	if _, err := fmt.Fprintf(stdout, "@%s %s\n", output.User.Atname, result); err != nil {
		slog.Error("結果の出力に失敗しました", "error", err)
		return 1
	}

	return 0
}
