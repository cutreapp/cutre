package invitation_acceptance

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/templates"
)

// Create POST /i/{token} (日本語版) と POST /en/i/{token} (英語版) - 受け取った招待をCookieに移し、登録の画面へ送る。
//
// 招待が使えるかは、表示したときから時間が経っているため改めて確かめる。
// 登録の画面は表示中の言語版へ送り、登録するアカウントの言語をこの画面の言語に揃える。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	output, ok := h.getInvitation(w, r, chi.URLParam(r, "token"))
	if !ok {
		return
	}

	h.continuationMgr.SetInvitationID(w, output.Invitation.ID)

	http.Redirect(w, r, templates.LocalePath(r.Context(), templates.SignUpPath), http.StatusSeeOther)
}
