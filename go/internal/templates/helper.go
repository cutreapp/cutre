// Package templates はtemplテンプレートから呼び出すヘルパーを提供する。
package templates

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/i18n"
)

// T はctxのロケールでmessageIDを翻訳する。
// テンプレートがi18nではなくtemplatesパッケージに依存するようにするためのi18n.Tの薄いラッパー。
func T(ctx context.Context, messageID string, templateData ...map[string]any) string {
	return i18n.T(ctx, messageID, templateData...)
}

// Locale はctxのロケールを返す。html要素のlang属性に使う。
// lang属性が取るのはBCP 47の言語タグで、ロケールはその綴りをそのまま持つため値は加工せずマークアップへ入る。
func Locale(ctx context.Context) string {
	return i18n.GetLocale(ctx)
}

// RootPath はctxのロケールの言語版のトップページのパスを返す。
// 言語版ごとにトップページのアドレスが違うため、行き先をテンプレートに直書きすると
// 英語版から日本語版のトップページへ送ることになる。
func RootPath(ctx context.Context) string {
	return i18n.LocalePath(i18n.GetLocale(ctx), "/")
}
