package model

import (
	"errors"
	"fmt"
)

// ValidationError は入力バリデーションの失敗の集まり。
// これを受け取ったハンドラーは、送信されたフォームをメッセージ付きで再描画する (422)。
type ValidationError struct {
	// Global はフォーム全体に関わるメッセージ。
	Global []string
	// Fields はフィールドごとのメッセージ。
	Fields map[string][]string
}

// NewValidationError は空の ValidationError を生成する。
func NewValidationError() *ValidationError {
	return &ValidationError{
		Global: []string{},
		Fields: map[string][]string{},
	}
}

// Error は ValidationError を error インターフェースに適合させる。
// 利用者に見せるのは Global / Fields のメッセージのため、ここでは固定の内部文字列を返す。
func (e *ValidationError) Error() string { return "バリデーションに失敗しました" }

// AddGlobal はフォーム全体のエラーメッセージを追加する。
func (e *ValidationError) AddGlobal(message string) {
	e.Global = append(e.Global, message)
}

// AddField は指定したフィールドのエラーメッセージを追加する。
func (e *ValidationError) AddField(field, message string) {
	if e.Fields == nil {
		e.Fields = map[string][]string{}
	}
	e.Fields[field] = append(e.Fields[field], message)
}

// HasErrors はエラーが1つでも追加されているかを返す。
func (e *ValidationError) HasErrors() bool {
	if e == nil {
		return false
	}

	return len(e.Global) > 0 || len(e.Fields) > 0
}

// HasFieldError は指定したフィールドにエラーがあるかを返す。
func (e *ValidationError) HasFieldError(field string) bool {
	return len(e.GetFieldErrors(field)) > 0
}

// GetFieldErrors は指定したフィールドのエラーメッセージを返す。
func (e *ValidationError) GetFieldErrors(field string) []string {
	if e == nil {
		return nil
	}

	return e.Fields[field]
}

// AppErrorCode はアプリケーションエラーの種別。
// ハンドラーがHTTPステータスコードを決めるために使う。
type AppErrorCode int

const (
	// AppErrCodeResourceNotFound はリソース未存在 (404相当)。
	AppErrCodeResourceNotFound AppErrorCode = iota + 1
	// AppErrCodeForbidden は権限不足 (403相当)。
	AppErrCodeForbidden
	// AppErrCodeConflict は状態の競合 (409相当)。
	AppErrCodeConflict
	// AppErrCodeInternal は想定済みの内部エラー (500相当)。
	AppErrCodeInternal
)

// AppError は業務レベルの既知の失敗を表す。
// Error() が返すのはユーザーに見せて安全なメッセージだけのため、
// error インターフェース経由で内部の原因が漏れることはない。
type AppError struct {
	// Code はハンドラーがステータスコードを決めるためのエラー種別。
	Code AppErrorCode
	// UserMsg は利用者に見せるメッセージ。内部情報を含めてはならない。
	UserMsg string
	// Internal はログに出す内部エラー。利用者には見せない。
	Internal error
	// Metadata は構造化ログに載せるコンテキスト (user_idなど)。
	Metadata map[string]string
}

// Error はユーザーに見せて安全なメッセージのみを返す。
func (e *AppError) Error() string { return e.UserMsg }

// Unwrap は内部エラーを errors.Is / errors.As のチェーンに載せる。
func (e *AppError) Unwrap() error { return e.Internal }

// LogString は内部の原因とメタデータを含む、ログ専用の詳細表現を返す。
func (e *AppError) LogString() string {
	return fmt.Sprintf("Code: %d | Msg: %s | Cause: %v | Meta: %v", e.Code, e.UserMsg, e.Internal, e.Metadata)
}

// AsValidationError はerrから *ValidationError を取り出す。
// errがそれでない (ラップもしていない) 場合はnilを返す。
func AsValidationError(err error) *ValidationError {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve
	}

	return nil
}

// AsAppError はerrから *AppError を取り出す。
// errがそれでない (ラップもしていない) 場合はnilを返す。
func AsAppError(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}

	return nil
}
