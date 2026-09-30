package httpadmin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/httpadmin"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TestParseIDParam は、URLのパラメータをUUIDとして読んでIDの型にし、読めない値ではfalseを返すことを検証する。
func TestParseIDParam(t *testing.T) {
	t.Parallel()

	want := uuid.MustParse("0192f0c6-1234-7abc-8def-0123456789ab")
	tests := []struct {
		name   string
		param  string
		wantID model.EventID
		wantOK bool
	}{
		{name: "UUID", param: want.String(), wantID: model.EventID(want), wantOK: true},
		{name: "UUIDとして読めない", param: "not-a-uuid", wantOK: false},
		{name: "空", param: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", tt.param)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

			id, ok := httpadmin.ParseIDParam[model.EventID](req, "id")
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("ParseIDParam() = (%s, %v)、期待値 = (%s, %v)", id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

// TestIsNotFound は、管理画面を使えないときと対象が無いときだけを404で応えるエラーとすることを検証する。
func TestIsNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "Forbidden", err: &model.AppError{Code: model.AppErrCodeForbidden}, want: true},
		{name: "ResourceNotFound", err: &model.AppError{Code: model.AppErrCodeResourceNotFound}, want: true},
		{name: "Conflict", err: &model.AppError{Code: model.AppErrCodeConflict}, want: false},
		{name: "AppErrorでない", err: errors.New("失敗"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := httpadmin.IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v、期待値 = %v", got, tt.want)
			}
		})
	}
}
