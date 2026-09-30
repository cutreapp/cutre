package settings_withdrawal

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/settings_withdrawal"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /settings/withdrawal - 退会の説明とフォームを描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の退会かを決められないまま描画しない。
		slog.ErrorContext(ctx, "退会の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderNew(w, r, user, http.StatusOK, false, nil)
}

// renderNew は退会の画面を指定したステータスで描画する。
// 退会のフォームを受け付けなかったとき (422・429) にも、エラーと一緒に描き直すのに使う。
// パスワードは入力欄に戻さない。値付きで描き直すと、応答やBFCacheに残るため。
// 進行中の交換があれば、その数と交換への案内を出し、退会するボタンを押せないようにする。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, user *model.User, status int, confirmed bool, formErrors *model.ValidationError) {
	ctx := r.Context()
	output, err := h.getWithdrawalUC.Execute(ctx, usecase.GetWithdrawalInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "退会の画面の情報の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "settings_withdrawal_new_title")

	data := page.NewPageData{
		ProfilePath: templates.ProfilePath(user.Atname),
		CSRFToken:   middleware.CSRFTokenFromContext(ctx),
		Confirmed:   confirmed,
		FormErrors:  formErrors,
		// 画面を開いたあとに交換が進んでも、退会のUseCaseがトランザクションの中で確かめ直す。
		InProgressTradeCount: output.InProgressTradeCount,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:    meta,
		MainNav: &components.MainNavData{Atname: user.Atname, Current: components.MainNavMyPage, CurrentPath: templates.SettingsWithdrawalPath},
	}
	if err := layouts.Default(layoutData, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "退会の画面の描画に失敗しました", "error", err)
	}
}
