// Package metrics はアプリのメトリクスを集め、Prometheusの形式で返す。
//
// 返す先は同じVPSのテレメトリーの収集 (vmagent) だけを想定し、公開側のルーターには載せない。
// 設定もHTTPのルーティングも知らず、集める対象 (接続プールなど) は呼び出し元から受け取る。
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Path はメトリクスを返すパス。
const Path = "/metrics"

// Registry はプロセスが公開するメトリクスをまとめて保持する。
//
// プロセス全体で共有する既定のレジストリ (prometheus.DefaultRegisterer) は使わない。
// テストごとに独立したレジストリを作れるようにし、同じメトリクスの二重登録で失敗しないようにするため。
type Registry struct {
	reg *prometheus.Registry
}

// NewRegistry はGoのランタイムとプロセスのメトリクスを登録したレジストリを返す。
func NewRegistry() *Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return &Registry{reg: reg}
}

// Handler は Path でメトリクスを返し、それ以外のパスには404を返すハンドラーを返す。
func (r *Registry) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET "+Path, promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{}))

	return mux
}
