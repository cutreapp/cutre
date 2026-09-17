// Package httperror は全リソース共通のHTTPエラーレスポンスを描画する。
//
// ここが応じるのはどのリソースディレクトリも持たないリクエストへの応答であり、
// internal/handler の下に置くとそのディレクトリのファイル名の規約に例外を作ることになるため、
// handlerの外のPresentation層ヘルパーとして置く。
package httperror

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	errorpages "github.com/cutreapp/cutre/go/internal/templates/pages/errors"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Renderer は共通のエラーページを描画する。
// 設定を保持するのは各ページのハンドラーと同じ理由で、共通レイアウトの<head>が
// 現在のアセットバージョンで静的アセットを参照するため。
type Renderer struct {
	cfg *config.Config
}

// NewRenderer は新しいRendererを作成する。
func NewRenderer(cfg *config.Config) *Renderer {
	return &Renderer{cfg: cfg}
}

// errorPage は1つのエラーページの応答を構成する要素。
// ステータスと文言と本文をまとめて渡すことで、応答を組み立てる手順自体は respond に1つだけ持たせる。
type errorPage struct {
	// status は応答するHTTPステータスコード。
	status int

	// titleKey は<title>と見出しに出す文言のキー。
	titleKey string

	// messageKey は説明文とmeta descriptionに出す文言のキー。
	messageKey string

	// content は本文を描画するコンポーネント。
	content templ.Component
}

// NotFound は404ページを応答する。
// ルーターのnot-foundハンドラーとして登録し、どのルートにも一致しないリクエストに応じる。
// 置き換えるchiの既定は、そこから先へ進む手段の無い平文1行である。
func (rd *Renderer) NotFound(w http.ResponseWriter, r *http.Request) {
	rd.respond(w, r, errorPage{
		status:     http.StatusNotFound,
		titleKey:   "error_not_found_title",
		messageKey: "error_not_found_message",
		content:    errorpages.NotFound(),
	})
}

// MethodNotAllowed は405ページを応答する。
// ルーターのmethod-not-allowedハンドラーとして登録し、アドレスは存在するがその方法では受け付けないリクエストに応じる。
// 置き換えるchiの既定はボディを持たない405で、404と同じく先へ進む手段が無い。
//
// Allowヘッダーはここでは付けない。どのメソッドを受け付けるかを知っているのはルーターであり、
// 付与は登録側 (cmd/cutre/serve.go) が行う。
func (rd *Renderer) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	rd.respond(w, r, errorPage{
		status:     http.StatusMethodNotAllowed,
		titleKey:   "error_method_not_allowed_title",
		messageKey: "error_method_not_allowed_message",
		content:    errorpages.MethodNotAllowed(),
	})
}

// respond はエラーページをバッファ上で組み立ててから、ステータスとボディをまとめて書き出す。
//
// ページはwへ何かが届く前に組み立てる。描画に失敗しても平文のエラーで応答を完結できるようにするため。
// wへ直接描画すると、失敗が表面化した時点でヘッダーと途中までのボディを送信済みになる。
func (rd *Renderer) respond(w http.ResponseWriter, r *http.Request, page errorPage) {
	ctx := r.Context()

	// エラー応答をキャッシュのヒューリスティクスに委ねず、方針を明示する。
	// このアドレスに後からページを追加したとき、保存されたエラー応答がそれを覆い隠さないようにするため。
	w.Header().Set("Cache-Control", "private, no-store")

	meta := viewmodel.ErrorPageMeta(ctx, rd.cfg)
	meta.SetTitle(ctx, page.titleKey)
	meta.Description = i18n.T(ctx, page.messageKey)

	var body bytes.Buffer
	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, page.content).Render(ctx, &body); err != nil {
		slog.ErrorContext(ctx, "エラーページの描画に失敗しました", "error", err, "status", page.status)
		http.Error(w, http.StatusText(page.status), page.status)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(page.status)

	if _, err := w.Write(body.Bytes()); err != nil {
		slog.ErrorContext(ctx, "エラーページの書き込みに失敗しました", "error", err, "status", page.status)
	}
}
