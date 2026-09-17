package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestRedirectBy は、クライアントを別の場所へ送るレスポンスが発行元を示すこと、
// 送らないレスポンスがその主張を伴わないことを検証する。
//
// 301は末尾スラッシュの正規化が生むため、現時点でアプリケーションが発行するリダイレクトは
// 手書きのステータスではなく実際の発行元で覆われている。
// 304はリダイレクトではない3xxであり、ステータスの範囲だけで判定すると印を付けてしまうため置いている。
func TestRedirectBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		handler        http.Handler
		target         string
		wantStatus     int
		wantRedirectBy string
	}{
		{
			name:           "末尾スラッシュの正規化が出す301に発行元が付く",
			handler:        middleware.RedirectSlashes(http.NotFoundHandler()),
			target:         "/health/",
			wantStatus:     http.StatusMovedPermanently,
			wantRedirectBy: "cutre",
		},
		{
			name: "メソッドを保つ恒久リダイレクトにも付く",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/en", http.StatusPermanentRedirect)
			}),
			target:         "/old",
			wantStatus:     http.StatusPermanentRedirect,
			wantRedirectBy: "cutre",
		},
		{
			name: "ページはリダイレクトではない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			target:     "/",
			wantStatus: http.StatusOK,
		},
		{
			name: "ステータスを設定せず本文を書いた応答はリダイレクトではない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("<!doctype html>")); err != nil {
					t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
				}
			}),
			target:     "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "エラーページはリダイレクトではない",
			handler:    http.NotFoundHandler(),
			target:     "/missing",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "行き先を持たない3xxはリダイレクトではない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotModified)
			}),
			target:     "/static/css/style.css",
			wantStatus: http.StatusNotModified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			middleware.RedirectBy(tt.handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Redirect-By"); got != tt.wantRedirectBy {
				t.Errorf("Redirect-By = %q、期待値 = %q", got, tt.wantRedirectBy)
			}
		})
	}
}

// TestRedirectBy_UnwrapsUnderlyingWriter は、本ミドルウェアが下へ渡すResponseWriterが
// サーバー自身のResponseWriterを露出し続けることを検証する。
// http.ResponseController がFlushなどの追加インターフェースへ到達するのに必要なものであり、
// このラッパーは全ルートを覆うため、失われるとサイト全体で利かなくなる。
func TestRedirectBy_UnwrapsUnderlyingWriter(t *testing.T) {
	t.Parallel()

	var flushed bool
	handler := middleware.RedirectBy(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("レスポンスのフラッシュのエラー = %v", err)
			return
		}
		flushed = true
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !flushed {
		t.Error("下層のFlusherへ到達できなかった")
	}
}

// TestRedirectBy_UsesUnderlyingReadFrom は、本文の書き出しが下層のResponseWriterの
// io.ReaderFrom へ渡ることを検証する。
//
// net/httpのResponseWriterはそこでsendfileを使い、静的アセットの配信がこの経路を通る。
// このラッパーが io.ReaderFrom を隠すと io.Copy から見えなくなり、経路が変わる。
func TestRedirectBy_UsesUnderlyingReadFrom(t *testing.T) {
	t.Parallel()

	const wantBody = "body{color:red}"

	rec := &readFromRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler := middleware.RedirectBy(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// http.ServeContentがio.CopyNでファイルを渡す形に合わせる。
		// strings.ReaderのWriteToが先に使われると、ReadFromを通る経路にならない。
		body := io.LimitReader(strings.NewReader(wantBody), int64(len(wantBody)))
		if _, err := io.Copy(w, body); err != nil {
			t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil))

	if !rec.readFromCalled {
		t.Error("下層のio.ReaderFromへ到達できなかった")
	}
	if got := rec.Body.String(); got != wantBody {
		t.Errorf("本文 = %q、期待値 = %q", got, wantBody)
	}
}

// TestRedirectBy_CopiesWithoutUnderlyingReadFrom は、下層が io.ReaderFrom を実装しないときも
// 本文がそのまま書き出されることを検証する。
func TestRedirectBy_CopiesWithoutUnderlyingReadFrom(t *testing.T) {
	t.Parallel()

	const wantBody = "body{color:red}"

	rec := httptest.NewRecorder()
	handler := middleware.RedirectBy(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// http.ServeContentがio.CopyNでファイルを渡す形に合わせる。
		// strings.ReaderのWriteToが先に使われると、ReadFromを通る経路にならない。
		body := io.LimitReader(strings.NewReader(wantBody), int64(len(wantBody)))
		if _, err := io.Copy(w, body); err != nil {
			t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil))

	if got := rec.Body.String(); got != wantBody {
		t.Errorf("本文 = %q、期待値 = %q", got, wantBody)
	}
}

// readFromRecorder は io.ReaderFrom を実装するResponseWriter。
// 本文の書き出しを自身のReadFromで受け取る点を、net/httpのResponseWriterと揃えている。
type readFromRecorder struct {
	*httptest.ResponseRecorder

	readFromCalled bool
}

func (rec *readFromRecorder) ReadFrom(r io.Reader) (int64, error) {
	rec.readFromCalled = true

	return io.Copy(rec.Body, r)
}
