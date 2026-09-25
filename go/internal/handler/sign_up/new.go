package sign_up

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	signuppage "github.com/cutreapp/cutre/go/internal/templates/pages/sign_up"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /sign_up (日本語版) と GET /en/sign_up (英語版) - メールアドレスの入力画面を描画する。
//
// 使える招待を持たずに開いた人には、フォームの代わりに招待制であることを示す。
// 招待を持たない訪問者にとっても正当な入口のため、200で返してインデックスも許す。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, hasInvitation, ok := h.usableInvitationID(w, r)
	if !ok {
		return
	}

	h.render(w, r, http.StatusOK, signuppage.NewPageData{
		InvitationRequired: !hasInvitation,
		CSRFToken:          middleware.CSRFTokenFromContext(ctx),
	})
}

// usableInvitationID はCookieが運ぶ招待が登録に使えるとき、その招待のIDを返す。
// 第2の戻り値は招待が使えるかどうかを表す。
// 招待を引けずに応答を書き終えたときは、第3の戻り値がfalseになる。
//
// 使えない招待のCookieは消す。残すと、期限が切れた招待を持ったまま画面を開き直すたびに同じ問い合わせが走る。
func (h *Handler) usableInvitationID(w http.ResponseWriter, r *http.Request) (model.InvitationID, bool, bool) {
	ctx := r.Context()

	invitationID, ok := h.continuationMgr.InvitationID(r)
	if !ok {
		return model.InvitationID{}, false, true
	}

	if _, err := h.getInvitationByIDUC.Execute(ctx, invitationID); err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.continuationMgr.DeleteInvitationID(w)
			return model.InvitationID{}, false, true
		}
		slog.ErrorContext(ctx, "登録に使う招待の取得に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return model.InvitationID{}, false, false
	}

	return invitationID, true, true
}

// render は登録の画面を指定したステータスで描画する。
// 表示 (200) と、招待が無い送信 (403)・受け付けなかった送信の再描画 (422 / 429) で共有する。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, data signuppage.NewPageData) {
	ctx := r.Context()

	// サイトキーはリクエストによらず設定で決まるため、呼び出し側ごとではなくここで入れる。
	data.TurnstileSiteKey = h.cfg.TurnstileSiteKey

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.SignUpPath)
	if !data.InvitationRequired {
		meta.AddTurnstilePreconnect(data.TurnstileSiteKey)
	}
	meta.SetTitle(ctx, "sign_up_new_title")
	meta.Description = i18n.T(ctx, "sign_up_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, signuppage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "登録の画面の描画に失敗しました", "error", err)
	}
}
