package admin_event

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/admin_event"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /admin/events/new - 管理画面のイベントの作成のフォームを描画する。
// 管理画面を使えないユーザーには404を返す。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。管理画面を開けるかを決められないまま描画しない。
		slog.ErrorContext(ctx, "管理画面のイベントの作成の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 作成の画面は引くデータを持たないため、管理画面の入口と同じUseCaseで管理画面を使えるかを確かめる。
	if err := h.getAdminMenuUC.Execute(ctx, usecase.GetAdminMenuInput{User: user}); err != nil {
		h.respondError(w, r, err, "管理画面のイベントの作成の画面の確認に失敗しました")
		return
	}

	h.renderNew(w, r, user, http.StatusOK, viewmodel.EventForm{}, nil)
}

// renderNew はイベントの作成の画面を指定したステータスで描画する。
// 作成のフォームを受け付けなかったとき (422) にも、送られた値とエラーと一緒に描き直すのに使う。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, user *model.User, status int, form viewmodel.EventForm, formErrors *model.ValidationError) {
	ctx := r.Context()

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "admin_event_new_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.NewAdminEventPath},
	}
	data := page.NewPageData{
		ProfilePath: templates.ProfilePath(user.Atname),
		CSRFToken:   middleware.CSRFTokenFromContext(ctx),
		Form:        form,
		FormErrors:  formErrors,
	}
	if err := layouts.Default(layoutData, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "管理画面のイベントの作成の画面の描画に失敗しました", "error", err)
	}
}
