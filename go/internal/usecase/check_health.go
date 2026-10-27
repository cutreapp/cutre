package usecase

import (
	"context"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
)

// checkHealthTimeout はヘルスチェックでデータベースの応答を待つ上限。
// 接続プールが埋まっているときやデータベースが応答しないときにも、外形監視とデプロイの確認へ早く503を返すため。
const checkHealthTimeout = 2 * time.Second

// CheckHealthUsecase はアプリケーションが依存先へ届くかを確かめる。
type CheckHealthUsecase struct {
	healthRepo *repository.HealthRepository
}

// NewCheckHealthUsecase は CheckHealthUsecase を生成する。
func NewCheckHealthUsecase(healthRepo *repository.HealthRepository) *CheckHealthUsecase {
	return &CheckHealthUsecase{healthRepo: healthRepo}
}

// CheckHealthOutput はヘルスチェックの結果。
// DBError はデータベースに届かなかった原因で、ログに残すためにだけ持つ (応答には載せない)。
type CheckHealthOutput struct {
	DBHealthy bool
	DBError   error
}

// Execute はデータベースへ軽いクエリを往復させ、その結果を返す。
// 届かないことはヘルスチェックの結果として返し、エラーにはしない。
func (uc *CheckHealthUsecase) Execute(ctx context.Context) *CheckHealthOutput {
	ctx, cancel := context.WithTimeout(ctx, checkHealthTimeout)
	defer cancel()

	if err := uc.healthRepo.Ping(ctx); err != nil {
		return &CheckHealthOutput{DBHealthy: false, DBError: err}
	}

	return &CheckHealthOutput{DBHealthy: true}
}
