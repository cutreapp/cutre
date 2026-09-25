package session_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/session"
)

// flashReporter はcontextに載ったフラッシュメッセージを本文に書き出すハンドラー。
func flashReporter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flash := session.FlashFromContext(r.Context())
		if flash == nil {
			_, _ = w.Write([]byte("none"))
			return
		}

		_, _ = w.Write([]byte(string(flash.Type) + ":" + flash.Message))
	})
}

// requestWithFlash はフラッシュCookieを載せたリクエストを返す。
func requestWithFlash(value string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: session.FlashCookieName, Value: value})

	return req
}

// encodeFlash はフラッシュCookieに入る形式へメッセージを変換する。
func encodeFlash(t *testing.T, flash session.FlashMessage) string {
	t.Helper()

	data, err := json.Marshal(flash)
	if err != nil {
		t.Fatalf("テスト用フラッシュメッセージの組み立てに失敗しました: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(data)
}

// TestFlashManager_Set は、種類ごとの設定メソッドがCookieに正しい種類を載せることと、
// そのCookieがセッションCookieと同じ方針で発行されることを検証する。
func TestFlashManager_Set(t *testing.T) {
	t.Parallel()

	mgr := session.NewFlashManager()

	tests := []struct {
		name string
		set  func(w http.ResponseWriter, message string)
		want session.FlashType
	}{
		{name: "成功", set: mgr.SetSuccess, want: session.FlashSuccess},
		{name: "エラー", set: mgr.SetError, want: session.FlashError},
		{name: "警告", set: mgr.SetWarning, want: session.FlashWarning},
		{name: "お知らせ", set: mgr.SetInfo, want: session.FlashInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			tt.set(rec, "メッセージ")

			cookies := rec.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("Set-Cookieの数 = %d、期待値 = 1", len(cookies))
			}
			cookie := cookies[0]

			if cookie.Name != session.FlashCookieName {
				t.Errorf("Cookie名 = %q、期待値 = %q", cookie.Name, session.FlashCookieName)
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
			if cookie.MaxAge <= 0 {
				t.Errorf("MaxAge = %d、期待値 = 正の値", cookie.MaxAge)
			}

			if want := encodeFlash(t, session.FlashMessage{Type: tt.want, Message: "メッセージ"}); cookie.Value != want {
				t.Errorf("Cookieの値 = %q、期待値 = %q", cookie.Value, want)
			}
		})
	}
}

// TestFlashManager_Middleware は、フラッシュメッセージがcontextに載り、
// 一度表示したらCookieが消えることを検証する。
func TestFlashManager_Middleware(t *testing.T) {
	t.Parallel()

	mgr := session.NewFlashManager()
	value := encodeFlash(t, session.FlashMessage{Type: session.FlashSuccess, Message: "ログインしました"})

	rec := httptest.NewRecorder()
	mgr.Middleware(flashReporter()).ServeHTTP(rec, requestWithFlash(value))

	if got, want := rec.Body.String(), "success:ログインしました"; got != want {
		t.Errorf("本文 = %q、期待値 = %q", got, want)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookieの数 = %d、期待値 = 1", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Errorf("MaxAge = %d、期待値 = 負の値", cookies[0].MaxAge)
	}
}

// TestFlashManager_Middleware_NoFlash は、表示できるフラッシュが無いリクエストで
// contextに何も載らないことを検証する。
//
// 利用者が書き換えられるCookieのため、壊れた値や知らない種類はそのまま表示しない。
func TestFlashManager_Middleware_NoFlash(t *testing.T) {
	t.Parallel()

	mgr := session.NewFlashManager()

	tests := []struct {
		name string
		req  *http.Request
	}{
		{name: "Cookieが無い", req: httptest.NewRequest(http.MethodGet, "/", nil)},
		{name: "デコードできない値", req: requestWithFlash("!!!")},
		{name: "JSONとして読めない値", req: requestWithFlash(base64.RawURLEncoding.EncodeToString([]byte("plain")))},
		{
			name: "知らない種類",
			req:  requestWithFlash(encodeFlash(t, session.FlashMessage{Type: "danger", Message: "メッセージ"})),
		},
		{
			name: "メッセージが空",
			req:  requestWithFlash(encodeFlash(t, session.FlashMessage{Type: session.FlashSuccess})),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			mgr.Middleware(flashReporter()).ServeHTTP(rec, tt.req)

			if got := rec.Body.String(); got != "none" {
				t.Errorf("本文 = %q、期待値 = %q", got, "none")
			}
		})
	}
}

// TestFlashManager_Middleware_ShowsOnlyOnce は、消去したCookieを送らない次のリクエストに
// 同じメッセージが現れないことを検証する。
func TestFlashManager_Middleware_ShowsOnlyOnce(t *testing.T) {
	t.Parallel()

	mgr := session.NewFlashManager()
	value := encodeFlash(t, session.FlashMessage{Type: session.FlashInfo, Message: "お知らせ"})

	handler := mgr.Middleware(flashReporter())

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, requestWithFlash(value))
	if got, want := first.Body.String(), "info:お知らせ"; got != want {
		t.Errorf("1回目の本文 = %q、期待値 = %q", got, want)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := second.Body.String(); got != "none" {
		t.Errorf("2回目の本文 = %q、期待値 = %q", got, "none")
	}
}

// TestFlashFromContext は、ミドルウェアを通っていないcontextからnilが返ることを検証する。
func TestFlashFromContext(t *testing.T) {
	t.Parallel()

	if flash := session.FlashFromContext(t.Context()); flash != nil {
		t.Errorf("FlashFromContext() = %v、期待値 = nil", flash)
	}
}
