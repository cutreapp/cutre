package model_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestValidationError_Collect はフォーム全体とフィールドのエラーが、
// 追加した先ごとに取り出せることを検証する。
func TestValidationError_Collect(t *testing.T) {
	t.Parallel()

	ve := model.NewValidationError()

	if ve.HasErrors() {
		t.Error("生成直後の HasErrors() = true、期待値 = false")
	}

	ve.AddGlobal("メールアドレスかパスワードが違います")
	ve.AddField("email", "メールアドレスを入力してください")
	ve.AddField("email", "メールアドレスの形式が正しくありません")

	if !ve.HasErrors() {
		t.Error("HasErrors() = false、期待値 = true")
	}

	if len(ve.Global) != 1 {
		t.Errorf("Globalの件数 = %d、期待値 = 1", len(ve.Global))
	}

	if !ve.HasFieldError("email") {
		t.Error("HasFieldError(\"email\") = false、期待値 = true")
	}

	if got := ve.GetFieldErrors("email"); len(got) != 2 {
		t.Errorf("GetFieldErrors(\"email\")の件数 = %d、期待値 = 2", len(got))
	}

	if ve.HasFieldError("password") {
		t.Error("エラーを追加していない password の HasFieldError() = true、期待値 = false")
	}
}

// TestValidationError_NilReceiver は、まだエラーが無いことをnilで表す呼び出し側のために、
// nilのレシーバでも参照系のメソッドが安全に働くことを検証する。
func TestValidationError_NilReceiver(t *testing.T) {
	t.Parallel()

	var ve *model.ValidationError

	if ve.HasErrors() {
		t.Error("nilレシーバの HasErrors() = true、期待値 = false")
	}

	if ve.HasFieldError("email") {
		t.Error("nilレシーバの HasFieldError() = true、期待値 = false")
	}

	if got := ve.GetFieldErrors("email"); got != nil {
		t.Errorf("nilレシーバの GetFieldErrors() = %v、期待値 = nil", got)
	}
}

// TestValidationError_AddFieldWithZeroValue は、構造体リテラルで生成して Fields がnilの
// ValidationError でも、フィールドのエラーを追加できることを検証する。
func TestValidationError_AddFieldWithZeroValue(t *testing.T) {
	t.Parallel()

	ve := &model.ValidationError{}
	ve.AddField("atname", "アットネームを入力してください")

	if got := ve.GetFieldErrors("atname"); len(got) != 1 {
		t.Errorf("GetFieldErrors(\"atname\")の件数 = %d、期待値 = 1", len(got))
	}
}

// TestAsValidationError は、ラップされた ValidationError も取り出せること、
// 無関係なエラーからはnilが返ることを検証する。
func TestAsValidationError(t *testing.T) {
	t.Parallel()

	ve := model.NewValidationError()
	ve.AddGlobal("入力を確認してください")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "ValidationErrorをそのまま渡す", err: ve, want: true},
		{name: "ラップされたValidationErrorを渡す", err: fmt.Errorf("保存に失敗: %w", ve), want: true},
		{name: "無関係なエラーを渡す", err: errors.New("接続に失敗"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := model.AsValidationError(tt.err)
			if (got != nil) != tt.want {
				t.Errorf("AsValidationError() = %v、期待値の非nil = %t", got, tt.want)
			}
		})
	}
}

// TestAppError_SafeMessage は、Error()が利用者向けのメッセージだけを返し、
// 内部の原因はログ用の表現にのみ現れることを検証する。
func TestAppError_SafeMessage(t *testing.T) {
	t.Parallel()

	internal := errors.New("pq: duplicate key value violates unique constraint")
	appErr := &model.AppError{
		Code:     model.AppErrCodeConflict,
		UserMsg:  "すでに使われています",
		Internal: internal,
		Metadata: map[string]string{"user_id": "01234567-89ab-7cde-8f01-23456789abcd"},
	}

	if got := appErr.Error(); got != "すでに使われています" {
		t.Errorf("Error() = %q、期待値 = %q", got, "すでに使われています")
	}

	if strings.Contains(appErr.Error(), "duplicate key") {
		t.Errorf("Error() = %q、内部の原因を含まないことを期待", appErr.Error())
	}

	if !errors.Is(appErr, internal) {
		t.Error("errors.Is(appErr, internal) = false、期待値 = true")
	}

	log := appErr.LogString()
	for _, want := range []string{"すでに使われています", "duplicate key", "user_id"} {
		if !strings.Contains(log, want) {
			t.Errorf("LogString() = %q、%qを含むことを期待", log, want)
		}
	}
}

// TestAsAppError は、ラップされた AppError も取り出せること、
// 無関係なエラーからはnilが返ることを検証する。
func TestAsAppError(t *testing.T) {
	t.Parallel()

	appErr := &model.AppError{Code: model.AppErrCodeResourceNotFound, UserMsg: "見つかりません"}

	if got := model.AsAppError(fmt.Errorf("取得に失敗: %w", appErr)); got == nil {
		t.Error("ラップされたAppErrorの AsAppError() = nil、非nilを期待")
	} else if got.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("Code = %d、期待値 = %d", got.Code, model.AppErrCodeResourceNotFound)
	}

	if got := model.AsAppError(errors.New("接続に失敗")); got != nil {
		t.Errorf("無関係なエラーの AsAppError() = %v、期待値 = nil", got)
	}
}
