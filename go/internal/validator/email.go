package validator

import (
	"context"
	"net/mail"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// validateEmail はメールアドレスが未入力か、アドレスとして読めないときに email のエラーを記録する。
// ログイン・登録・パスワードリセットの申請のフォームで同じ規則を使う。
//
// net/mail は "名前 <a@example.com>" のような表示名付きの形も受け付けるため、
// 読み取ったアドレスが入力そのものと一致することまで確かめる。
func validateEmail(ctx context.Context, ve *model.ValidationError, email string) {
	if email == "" {
		ve.AddField("email", i18n.T(ctx, "validation_required"))
		return
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		ve.AddField("email", i18n.T(ctx, "validation_email_invalid"))
	}
}
