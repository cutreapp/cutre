package usecase

import "context"

// SetBeforeEnableHook は、EnableTwoFactorAuthUsecase が照合の後、有効にする前に呼ぶ関数を差し込む。
// 照合と有効化の間に別の画面で秘密鍵が差し替わる競合を、テストで決まった順に起こすために使う。
func (uc *EnableTwoFactorAuthUsecase) SetBeforeEnableHook(hook func(ctx context.Context)) {
	uc.beforeEnable = hook
}

// SetBeforeDisableHook は、DisableTwoFactorAuthUsecase が再認証の後、削除の前に呼ぶ関数を差し込む。
// その間に設定が入れ替わる競合を、テストで決まった順に起こすために使う。
func (uc *DisableTwoFactorAuthUsecase) SetBeforeDisableHook(hook func(ctx context.Context)) {
	uc.beforeDisable = hook
}
