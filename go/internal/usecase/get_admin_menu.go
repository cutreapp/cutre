package usecase

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
)

// GetAdminMenuUsecase は管理画面の入口を開けるかを確かめる。
// 入口はマスタの種類ごとの画面へのリンクを並べるだけで、引くデータは無い。
type GetAdminMenuUsecase struct{}

// NewGetAdminMenuUsecase は GetAdminMenuUsecase を生成する。
func NewGetAdminMenuUsecase() *GetAdminMenuUsecase {
	return &GetAdminMenuUsecase{}
}

// GetAdminMenuInput は GetAdminMenuUsecase.Execute の入力。
type GetAdminMenuInput struct {
	User *model.User
}

// Execute は、ユーザーが管理画面を使えないときに AppErrCodeForbidden の *model.AppError を返す。
func (uc *GetAdminMenuUsecase) Execute(_ context.Context, input GetAdminMenuInput) error {
	return authorizeAdmin(input.User)
}
