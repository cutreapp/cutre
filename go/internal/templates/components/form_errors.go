package components

import (
	"fmt"
	"strings"

	"github.com/cutreapp/cutre/go/internal/model"
)

// fieldErrorID はフィールドのエラーメッセージを入れる要素のidを返す。
// 入力欄の aria-describedby から参照し、支援技術が入力欄とエラーを結び付けられるようにする。
func fieldErrorID(field string, i int) string {
	return fmt.Sprintf("%s-error-%d", field, i)
}

// FieldErrorsDescribedBy はフィールドのエラーメッセージのidを空白区切りで返す。
// 入力欄の aria-describedby に渡す。エラーが無いときは空文字列を返す。
func FieldErrorsDescribedBy(field string, formErrors *model.ValidationError) string {
	messages := formErrors.GetFieldErrors(field)
	ids := make([]string, len(messages))
	for i := range messages {
		ids[i] = fieldErrorID(field, i)
	}
	return strings.Join(ids, " ")
}

// FormErrorSummaryField はエラーの要約に並べるフィールド。
type FormErrorSummaryField struct {
	// Name は入力欄のidとname。要約のリンクはこのidへ移動する。
	Name string
	// LabelKey は入力欄のラベルの翻訳キー。
	LabelKey string
}

// FormErrorSummaryData はエラーの要約の描画に使うデータ。
type FormErrorSummaryData struct {
	// Fields は要約に並べるフィールドを、フォームに現れる順で持つ。
	Fields []FormErrorSummaryField
	Errors *model.ValidationError
}

// hasEntries は要約に並べるフィールドのエラーが1件でもあるかを返す。
func (d FormErrorSummaryData) hasEntries() bool {
	for _, field := range d.Fields {
		if d.Errors.HasFieldError(field.Name) {
			return true
		}
	}
	return false
}
