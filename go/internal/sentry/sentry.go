// Package sentry はエラートラッカーのSentryとの連携を提供する。
package sentry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

// Config はSentryの設定を保持する。
type Config struct {
	DSN              string
	Environment      string
	Release          string
	TracesSampleRate float64
	Debug            bool
}

// maskedValue は送る前に伏せた値の置き換え先。
const maskedValue = "[FILTERED]"

// sensitiveHeaders は伏せるHTTPヘッダーの名前 (小文字、完全一致)。
var sensitiveHeaders = []string{
	"authorization",
	"cookie",
	// 同一サイト内の遷移では、招待トークンを含むURLも載るため丸ごと伏せる。
	"referer",
	"x-csrf-token",
}

// sensitiveBodyKeys は伏せるリクエストボディのキー (小文字、部分一致)。
// code は確認コード・認証アプリのコード・リカバリーコード、credential は二要素認証の解除に使う資格情報。
var sensitiveBodyKeys = []string{
	"password",
	"token",
	"secret",
	"code",
	"credential",
}

// sensitiveQueryKeys は伏せるクエリパラメータのキー (小文字、部分一致)。
var sensitiveQueryKeys = []string{
	"token",
	"key",
}

// sensitiveTagKeys は伏せるタグのキー (小文字、部分一致)。
//
// slogのハンドラーはログの属性をそのままタグに載せるため、属性として書いた個人情報
// (メールの送信の失敗に添えた email など) が伏せられないまま届いてしまう。
// 標準エラー出力のログには元の値が残るため、調べるときはそちらを見る。
var sensitiveTagKeys = []string{
	"email",
	"password",
	"secret",
	"token",
}

// ignoredErrorPatterns はメッセージで送らないと判断するエラーのパターン (正規表現)。
// クライアントの切断や、Goのランタイムによる正常な中断を除く。
var ignoredErrorPatterns = []string{
	"context canceled",
	"net/http: abort Handler",
}

// Init はSentryを初期化する。
// DSNが空のときは何もせずnilを返す。開発環境とテスト環境はこの状態で動かす。
func Init(cfg Config) error {
	if cfg.DSN == "" {
		slog.Info("Sentry DSNが設定されていないため、Sentryは無効です")
		return nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:                   cfg.DSN,
		Environment:           cfg.Environment,
		Release:               cfg.Release,
		TracesSampleRate:      cfg.TracesSampleRate,
		EnableTracing:         true,
		Debug:                 cfg.Debug,
		BeforeSend:            beforeSend,
		BeforeSendTransaction: beforeSendTransaction,
		IgnoreErrors:          ignoredErrorPatterns,
	})
	if err != nil {
		return err
	}

	slog.Info("Sentryを初期化しました",
		"environment", cfg.Environment,
		"release", cfg.Release,
		"traces_sample_rate", cfg.TracesSampleRate,
	)
	return nil
}

// Flush は送信待ちのイベントを送り切る。アプリケーションの終了時に呼ぶ。
func Flush(timeout time.Duration) {
	sentry.Flush(timeout)
}

// beforeSend はエラーのイベントを送る前に、送らないエラーを捨て、残りの機微な値を伏せる。
func beforeSend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	if hint != nil && shouldDropError(hint.OriginalException) {
		return nil
	}

	filterEvent(event)
	return event
}

// beforeSendTransaction はパフォーマンスのトレースを送る前に、機微な値を伏せる。
// トレースにもリクエストの情報とスコープのタグが載るため、エラーのイベントと同じ扱いにする。
func beforeSendTransaction(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	filterEvent(event)
	return event
}

// filterEvent はイベントのタグとリクエストの情報から機微な値を伏せる。
func filterEvent(event *sentry.Event) {
	filterTags(event)

	if event.Request != nil {
		filterRequestHeaders(event.Request)
		filterRequestData(event.Request)
		filterQueryString(event.Request)
	}
}

// shouldDropError はクライアントの切断やランタイムの中断に由来するエラーかを返す。
// 該当するものはSentryに送らない。
func shouldDropError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, http.ErrAbortHandler) {
		return true
	}
	return false
}

// filterTags は機微なタグ (メールアドレスなど) を伏せる。
// リクエストの情報と違い、slogのハンドラーがログの属性から作ったタグも対象になる。
func filterTags(event *sentry.Event) {
	for key := range event.Tags {
		if containsAny(strings.ToLower(key), sensitiveTagKeys) {
			event.Tags[key] = maskedValue
		}
	}
}

// filterRequestHeaders は機微なHTTPヘッダーを伏せる。
func filterRequestHeaders(req *sentry.Request) {
	for name := range req.Headers {
		lowerName := strings.ToLower(name)
		for _, sensitive := range sensitiveHeaders {
			if lowerName == sensitive {
				req.Headers[name] = maskedValue
				break
			}
		}
	}
}

// filterRequestData は機微なリクエストボディのフィールドを伏せる。
// 読み取れないボディは、機微な値を含んでいないと言い切れないため丸ごと伏せる。
func filterRequestData(req *sentry.Request) {
	if req.Data == "" {
		return
	}

	filtered, ok := filterValues(req.Data, sensitiveBodyKeys)
	if !ok {
		req.Data = maskedValue
		return
	}
	req.Data = filtered
}

// filterQueryString は機微なクエリパラメータを伏せる。
// 読み取れないクエリは、機微な値を含んでいないと言い切れないため丸ごと伏せる。
func filterQueryString(req *sentry.Request) {
	if req.QueryString == "" {
		return
	}

	filtered, ok := filterValues(req.QueryString, sensitiveQueryKeys)
	if !ok {
		req.QueryString = maskedValue
		return
	}
	req.QueryString = filtered
}

// filterValues はURLエンコードされた値のうち、キーがsensitiveKeysのどれかを含むものを伏せる。
// 伏せるものが無いときは元の文字列をそのまま返す。読み取れないときはokをfalseにする。
func filterValues(encoded string, sensitiveKeys []string) (string, bool) {
	values, err := url.ParseQuery(encoded)
	if err != nil {
		return "", false
	}

	filtered := false
	for key := range values {
		if containsAny(strings.ToLower(key), sensitiveKeys) {
			values.Set(key, maskedValue)
			filtered = true
		}
	}

	if !filtered {
		return encoded, true
	}
	return values.Encode(), true
}

// containsAny はsがsubstrsのどれかを含むかを返す。
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
