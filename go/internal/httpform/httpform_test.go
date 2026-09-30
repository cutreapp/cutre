package httpform_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/httpform"
	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestLockVersion は、フォームの版を読み、無い・読めない版を MissingLockVersion にすることと、
// POSTに _method を載せたDELETEでも MethodOverride が読んだ版を返すことを検証する。
func TestLockVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want int32
	}{
		{name: "版あり", body: "lock_version=3", want: 3},
		{name: "版なし", body: "", want: httpform.MissingLockVersion},
		{name: "数値として読めない", body: "lock_version=x", want: httpform.MissingLockVersion},
		{name: "int32に収まらない", body: "lock_version=2147483648", want: httpform.MissingLockVersion},
		{name: "_methodでDELETEに上書き", body: "_method=DELETE&lock_version=2", want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			var got int32
			middleware.MethodOverride(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = httpform.LockVersion(r)
			})).ServeHTTP(httptest.NewRecorder(), req)

			if got != tt.want {
				t.Errorf("LockVersion() = %d、期待値 = %d", got, tt.want)
			}
		})
	}
}
