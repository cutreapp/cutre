package middleware

import (
	"net/http"
	"net/url"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/logging"
)

// SentryTransaction はSentryに送るトランザクションの名前とリクエストのURLを、chiのルートパターンで表す。
//
// パスではなくルートパターンを使うのは、アクセスログと同じく、パスに含まれるトークン (招待のURLなど) を
// 外部のサービスへ送らず、URLの値ごとにトランザクションが分かれないようにするため。
//
// 入口でリクエストのスコープにイベントの処理を仕込み、送る時点のルートパターンでイベントを書き換える。
// chiはハンドラーを呼ぶ前にルートパターンを決めるため、ハンドラーの中で送るイベントにもpanicのイベントにも効く。
// 出口では進行中のトランザクションの名前を書き換え、panic時のトレースを失敗として補正する。
//
// sentryhttpより内側に登録する。deferは後に登録したものから走るため、
// sentryhttpがトランザクションを終える前、およびpanicを捕捉する前に書き換えが済む。
func SentryTransaction(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.Scope().AddEventProcessor(sentryRouteEventProcessor(r))
		}
		// 応答済みのステータスは、panicのあとも実際のレスポンスに合わせて保持する。
		rw := &statusRecorder{ResponseWriter: w}
		defer func() {
			applySentryTransactionName(r)
			if recovered := recover(); recovered != nil {
				if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
					hub.Scope().AddEventProcessor(sentryPanicTransactionProcessor(recovered, rw.status))
				}
				panic(recovered)
			}
		}()
		next.ServeHTTP(rw, r)
	})
}

// sentryPanicTransactionProcessor はSDKがトランザクションを終えたあとのイベントを補正する。
// 外側のRecovererが500を書く前にSDKが完了するため、未応答のpanicを成功扱いさせない。
// ErrAbortHandlerは応答を返さず接続を中断するため、HTTPステータスを持つトレースとして送らない。
func sentryPanicTransactionProcessor(recovered any, status int) sentry.EventProcessor {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	return func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
		if event == nil || event.Type != "transaction" {
			return event
		}
		if recovered == http.ErrAbortHandler {
			return nil
		}
		trace := event.Contexts["trace"]
		trace["status"] = sentry.SpanStatusInternalError
		data, ok := trace["data"].(map[string]any)
		if !ok {
			data = make(map[string]any)
			trace["data"] = data
		}
		data["http.response.status_code"] = status
		return event
	}
}

// sentryRouteEventProcessor は、イベントのリクエストのURLのパスをルートパターンに置き換え、
// エラーのイベントのトランザクションをルートパターンで埋める処理を返す。
//
// 一致するルートが無いときは、パスを取り除く。404のパスにも利用者が持ってきたトークンが入りうるため。
// トレース (transaction) のトランザクションは、トランザクションの名前から決まるためここでは触らない。
func sentryRouteEventProcessor(r *http.Request) sentry.EventProcessor {
	return func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
		if event == nil {
			return nil
		}

		pattern := matchedRoutePattern(r)
		if event.Request != nil {
			event.Request.URL = replaceURLPath(event.Request.URL, pattern)
		}
		if event.Type != "transaction" && event.Transaction == "" && pattern != "" {
			event.Transaction = r.Method + " " + pattern
		}
		return event
	}
}

// replaceURLPath はrawURLのパスをpathに置き換える。読み取れないURLは丸ごと取り除く。
func replaceURLPath(rawURL, path string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	u.Path = path
	u.RawPath = ""
	return u.String()
}

// applySentryTransactionName は進行中のトランザクションの名前をルートパターンで書き換える。
// 一致するルートが無いときはメソッドだけにし、パスを送らない。
func applySentryTransactionName(r *http.Request) {
	transaction := sentry.TransactionFromContext(r.Context())
	if transaction == nil {
		return
	}

	pattern := matchedRoutePattern(r)
	if pattern == "" {
		transaction.Name = r.Method
		return
	}
	transaction.Name = r.Method + " " + pattern
	transaction.Source = sentry.SourceRoute
}

// matchedRoutePattern はchiが一致させたルートパターンを返す。chiのルーティングを通っていないときは空を返す。
func matchedRoutePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}
	return rctx.RoutePattern()
}

// SentryRequestID はリクエストIDをSentryのスコープのタグに載せ、イベントとログを突き合わせられるようにする。
// RequestIDより内側、かつリクエストのスコープを作るsentryhttpより内側に登録する。
func SentryRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			if id := logging.RequestIDFromContext(r.Context()); id != "" {
				hub.Scope().SetTag("request_id", id)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// SentryUserContext はログイン中のユーザーのIDをSentryのスコープに載せる。
//
// 送るのはIDだけにする。調べるときはIDからユーザーを引けるため、メールアドレスやアットネームを外部のサービスへ送らない。
// 現在のユーザーを解決する SetUser より内側に登録する。
func SentryUserContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user := UserFromContext(r.Context()); user != nil {
			if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
				hub.Scope().SetUser(sentry.User{ID: user.ID.String()})
			}
		}
		next.ServeHTTP(w, r)
	})
}
