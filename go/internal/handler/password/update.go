package password

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/password"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Update PATCH /password と PATCH /en/password - 新しいパスワードを設定し、ログイン画面へ送る。
//
// HTMLのフォームからは _method の上書きで届く。
// 受け付けなかった送信はフォームをメッセージ付きで再描画し、同じリンクのまま直して送り直せるようにする。
// 設定を終えてもログインさせない。メールを受け取れるだけでログインまで済ませられないよう、
// ログインはログイン画面の関門を通して行わせる。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	output, ok := h.usableToken(w, r)
	if !ok {
		return
	}

	err := h.updatePasswordUC.Execute(ctx, usecase.UpdatePasswordInput{
		PasswordResetTokenID: output.PasswordResetToken.ID,
		Password:             r.PostFormValue("password"),
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.render(w, r, http.StatusUnprocessableEntity, page.EditPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Email:      output.User.Email,
				FormErrors: ve,
			})
			return
		}
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.renderUnusable(w, r)
			return
		}
		slog.ErrorContext(ctx, "新しいパスワードの設定に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.continuationMgr.DeletePasswordResetTokenID(w)
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_password_updated"))

	http.Redirect(w, r, templates.LocalePath(ctx, templates.SignInPath), http.StatusSeeOther)
}
