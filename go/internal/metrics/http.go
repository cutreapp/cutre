package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// otherMethod は knownMethods に無いメソッドをまとめるラベルの値。
const otherMethod = "OTHER"

// knownMethods はラベルにそのまま使うメソッド。
// メソッドはクライアントが任意の文字列を送れるため、そのまま使うと時系列が際限なく増える。
var knownMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodPost:    {},
	http.MethodPut:     {},
	http.MethodPatch:   {},
	http.MethodDelete:  {},
	http.MethodOptions: {},
}

// HTTP はHTTPリクエストの件数と所要時間を記録する。
//
// 件数はヒストグラムの _count で数えられるため、別のカウンターは持たない。
type HTTP struct {
	duration *prometheus.HistogramVec
}

// NewHTTP はHTTPリクエストのメトリクスをレジストリに登録して返す。
func (r *Registry) NewHTTP() *HTTP {
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTPリクエストの処理にかかった時間 (秒)。ルートパターン・メソッド・ステータス別。",
		Buckets: prometheus.DefBuckets,
	}, []string{"route", "method", "status"})
	r.reg.MustRegister(duration)

	return &HTTP{duration: duration}
}

// Observe は処理を終えたリクエストを1件記録する。
//
// routeにはパスではなくルートパターンを渡す。パスごとに時系列を作ると際限なく増えるため。
// 一致するルートが無いときは空文字列を渡す (アクセスログと同じ)。
func (h *HTTP) Observe(route, method string, status int, elapsed time.Duration) {
	if _, ok := knownMethods[method]; !ok {
		method = otherMethod
	}

	h.duration.WithLabelValues(route, method, strconv.Itoa(status)).Observe(elapsed.Seconds())
}
