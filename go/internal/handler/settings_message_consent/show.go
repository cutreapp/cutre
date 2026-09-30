package settings_message_consent

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/settings_message_consent"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET /settings/message_consent - メッセージの取り扱いへの同意の状態を描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
//
// 有効な同意があれば同意した日と内容を、無ければ (やめた・古い版の文面に同意した・記録が無い) 今の文面と同意する入口を出す。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の同意かを決められないまま描画しない。
		slog.ErrorContext(ctx, "メッセージの利用の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderShow(w, r, user, http.StatusOK, nil)
}

// renderShow はメッセージの利用の画面を指定したステータスで描画する。
// 同意をやめられなかったとき (422) にも、エラーと一緒に描き直すのに使う。
func (h *Handler) renderShow(w http.ResponseWriter, r *http.Request, user *model.User, status int, formErrors *model.ValidationError) {
	ctx := r.Context()

	output, err := h.getMessageConsentUC.Execute(ctx, usecase.GetMessageConsentInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "メッセージの取り扱いへの同意の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := page.ShowPageData{
		ProfilePath: templates.ProfilePath(user.Atname),
		Agreed:      output.Consent != nil && output.Consent.IsValid(),
		CSRFToken:   middleware.CSRFTokenFromContext(ctx),
		FormErrors:  formErrors,
	}
	if data.Agreed {
		// 同意した日はユーザーのタイムゾーンの日付で示す。
		loc, err := user.Location()
		if err != nil {
			slog.ErrorContext(ctx, "ユーザーのタイムゾーンの読み込みに失敗しました", "error", err, "user_id", user.ID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		data.AgreedOn = viewmodel.FormatDate(ctx, output.Consent.AgreedAt.In(loc), time.Now().In(loc))
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "settings_message_consent_show_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.SettingsMessageConsentPath},
	}
	if err := layouts.Default(layoutData, page.Show(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "メッセージの利用の画面の描画に失敗しました", "error", err)
	}
}
