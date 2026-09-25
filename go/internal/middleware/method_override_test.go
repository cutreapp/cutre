package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestMethodOverride は、フォームが送れないメソッドへの書き換えと、
// 書き換えてはいけないリクエストをそのまま通すことを検証する。
func TestMethodOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		override   string
		wantMethod string
	}{
		{name: "PATCHへ書き換える", method: http.MethodPost, override: "PATCH", wantMethod: http.MethodPatch},
		{name: "DELETEへ書き換える", method: http.MethodPost, override: "DELETE", wantMethod: http.MethodDelete},
		{name: "PUTへ書き換える", method: http.MethodPost, override: "PUT", wantMethod: http.MethodPut},
		{name: "小文字でも書き換える", method: http.MethodPost, override: "delete", wantMethod: http.MethodDelete},
		{name: "指定が無ければPOSTのまま", method: http.MethodPost, override: "", wantMethod: http.MethodPost},
		{name: "未知の値は無視する", method: http.MethodPost, override: "TRACE", wantMethod: http.MethodPost},
		{name: "安全なメソッドへは書き換えない", method: http.MethodPost, override: "GET", wantMethod: http.MethodPost},
		{name: "POST以外は書き換えない", method: http.MethodPut, override: "DELETE", wantMethod: http.MethodPut},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			form := url.Values{middleware.MethodOverrideFieldName: {tt.override}}
			req := httptest.NewRequest(tt.method, "/settings/invitations/1", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			var gotMethod string
			middleware.MethodOverride(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
			})).ServeHTTP(httptest.NewRecorder(), req)

			if gotMethod != tt.wantMethod {
				t.Errorf("メソッド = %q、期待値 = %q", gotMethod, tt.wantMethod)
			}
		})
	}
}

// TestMethodOverride_KeepsFormValues は、書き換えのために本文を読んだあとも
// ハンドラーが他のフォームの値を読めることを検証する。
func TestMethodOverride_KeepsFormValues(t *testing.T) {
	t.Parallel()

	form := url.Values{
		middleware.MethodOverrideFieldName: {"DELETE"},
		"csrf_token":                       {"token"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/invitations/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var gotToken string
	middleware.MethodOverride(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotToken = r.PostFormValue("csrf_token")
	})).ServeHTTP(httptest.NewRecorder(), req)

	if gotToken != "token" {
		t.Errorf("csrf_token = %q、期待値 = %q", gotToken, "token")
	}
}
