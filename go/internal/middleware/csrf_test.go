package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

// csrfTokenReporter はcontextに載ったCSRFトークンを本文に書き出すハンドラー。
func csrfTokenReporter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(middleware.CSRFTokenFromContext(r.Context())))
	})
}

// postFormRequest はフォームを送信するPOSTリクエストを組み立てる。
func postFormRequest(form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

// TestCSRF_IssuesToken は、安全なリクエストがトークンを受け取り、
// そのトークンがCookieとcontextの両方に載ることを検証する。
func TestCSRF_IssuesToken(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	middleware.NewCSRF().Middleware(csrfTokenReporter()).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sign_in", nil))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookieの数 = %d、期待値 = 1", len(cookies))
	}
	cookie := cookies[0]

	if cookie.Name != middleware.CSRFCookieName {
		t.Errorf("Cookie名 = %q、期待値 = %q", cookie.Name, middleware.CSRFCookieName)
	}
	if cookie.Value == "" {
		t.Error("Cookieの値 = 空文字列、非空を期待")
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q、期待値 = %q", cookie.Path, "/")
	}
	if cookie.Domain != "" {
		t.Errorf("Domain = %q、期待値 = 空文字列", cookie.Domain)
	}
	if !cookie.Secure {
		t.Error("Secure = false、期待値 = true")
	}
	if !cookie.HttpOnly {
		t.Error("HttpOnly = false、期待値 = true")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v、期待値 = %v", cookie.SameSite, http.SameSiteLaxMode)
	}

	if got := rec.Body.String(); got != cookie.Value {
		t.Errorf("contextのトークン = %q、期待値 = %q", got, cookie.Value)
	}
}

// TestCSRF_ReusesToken は、既にトークンを持つ訪問者へ新しいトークンを発行し直さないことを検証する。
// リクエストごとに作り直すと、複数のタブで開いたフォームのうち古いものが通らなくなる。
func TestCSRF_ReusesToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/sign_in", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: "existing-token"})

	rec := httptest.NewRecorder()
	middleware.NewCSRF().Middleware(csrfTokenReporter()).ServeHTTP(rec, req)

	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("Set-Cookieの数 = %d、期待値 = 0", len(cookies))
	}
	if got := rec.Body.String(); got != "existing-token" {
		t.Errorf("contextのトークン = %q、期待値 = %q", got, "existing-token")
	}
}

// TestCSRF_AcceptsValidToken は、Cookieと一致するトークンを提示したリクエストが
// ハンドラーまで届き、検証したトークンをcontextから読めることを検証する。
// 提示の経路はフォームとヘッダーの2つを認める (htmxのようにフォームを持たない送信があるため)。
func TestCSRF_AcceptsValidToken(t *testing.T) {
	t.Parallel()

	const token = "valid-token"

	tests := []struct {
		name string
		req  *http.Request
	}{
		{
			name: "フォームのフィールドで提示する",
			req:  postFormRequest(url.Values{middleware.CSRFFieldName: {token}}),
		},
		{
			name: "ヘッダーで提示する",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/sign_in", nil)
				req.Header.Set(middleware.CSRFHeaderName, token)

				return req
			}(),
		},
		{
			name: "同一オリジンからの送信であることを名乗る",
			req: func() *http.Request {
				req := postFormRequest(url.Values{middleware.CSRFFieldName: {token}})
				req.Header.Set("Sec-Fetch-Site", "same-origin")

				return req
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})

			rec := httptest.NewRecorder()
			middleware.NewCSRF().Middleware(csrfTokenReporter()).ServeHTTP(rec, tt.req)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Body.String(); got != token {
				t.Errorf("contextのトークン = %q、期待値 = %q", got, token)
			}
		})
	}
}

// TestCSRF_RejectsInvalidRequest は、検証に通らないリクエストがハンドラーに到達せず
// 403で止まることを検証する。
func TestCSRF_RejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cookieValue string
		req         *http.Request
	}{
		{
			name: "Cookieが無い",
			req:  postFormRequest(url.Values{middleware.CSRFFieldName: {"some-token"}}),
		},
		{
			name:        "トークンを提示していない",
			cookieValue: "valid-token",
			req:         postFormRequest(url.Values{}),
		},
		{
			name:        "提示したトークンが一致しない",
			cookieValue: "valid-token",
			req:         postFormRequest(url.Values{middleware.CSRFFieldName: {"other-token"}}),
		},
		{
			name:        "クロスサイトからの送信",
			cookieValue: "valid-token",
			req: func() *http.Request {
				req := postFormRequest(url.Values{middleware.CSRFFieldName: {"valid-token"}})
				req.Header.Set("Sec-Fetch-Site", "cross-site")

				return req
			}(),
		},
		{
			name:        "別オリジンを名乗るOrigin",
			cookieValue: "valid-token",
			req: func() *http.Request {
				req := postFormRequest(url.Values{middleware.CSRFFieldName: {"valid-token"}})
				req.Header.Set("Origin", "https://attacker.example.com")

				return req
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cookieValue != "" {
				tt.req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: tt.cookieValue})
			}

			handlerCalled := false
			handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				handlerCalled = true
			})

			rec := httptest.NewRecorder()
			middleware.NewCSRF().Middleware(handler).ServeHTTP(rec, tt.req)

			if handlerCalled {
				t.Error("検証に通らないリクエストがハンドラーに到達した")
			}
			if rec.Code != http.StatusForbidden {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
			}
		})
	}
}

// TestCSRFTokenFromContext は、ミドルウェアを通っていないcontextから空文字列が返ることを検証する。
func TestCSRFTokenFromContext(t *testing.T) {
	t.Parallel()

	if got := middleware.CSRFTokenFromContext(t.Context()); got != "" {
		t.Errorf("CSRFTokenFromContext() = %q、期待値 = 空文字列", got)
	}
}
