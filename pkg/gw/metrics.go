/*
Copyright 2019-2020 vChain, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gw

import (
	"expvar"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/codenotary/immudb/pkg/api/schema"
	"github.com/codenotary/immudb/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsCollection struct {
	AuditResultPerServer   *prometheus.GaugeVec
	AuditPrevRootPerServer *prometheus.GaugeVec
	AuditCurrRootPerServer *prometheus.GaugeVec
	AuditRunAtPerServer    *prometheus.GaugeVec

	UptimeCounter prometheus.CounterFunc
}

var metricsNamespace = "immugw"

func (mc *MetricsCollection) WithUptimeCounter(f func() float64) {
	mc.UptimeCounter = promauto.NewCounterFunc(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "uptime_hours",
			Help:      "Immu gateway server uptime in hours.",
		},
		f,
	)
}

func (mc *MetricsCollection) UpdateAuditResult(
	serverID string,
	serverAddress string,
	result bool,
	prevRoot *schema.Root,
	currRoot *schema.Root,
) {
	var r float64
	if result {
		r = 1
	}
	mc.AuditResultPerServer.
		WithLabelValues(serverID, serverAddress).Set(r)
	mc.AuditPrevRootPerServer.
		WithLabelValues(serverID, serverAddress).Set(float64(prevRoot.GetIndex()))
	mc.AuditCurrRootPerServer.
		WithLabelValues(serverID, serverAddress).Set(float64(currRoot.GetIndex()))
	mc.AuditRunAtPerServer.
		WithLabelValues(serverID, serverAddress).SetToCurrentTime()
}

func newAuditGaugeVec(name string, help string) *prometheus.GaugeVec {
	return promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      name,
			Help:      help,
		},
		[]string{"server_id", "server_address"},
	)
}

var Metrics = MetricsCollection{
	AuditResultPerServer: newAuditGaugeVec(
		"audit_result_per_server",
		"Latest audit result (1 = ok, 0 = tampered).",
	),
	AuditPrevRootPerServer: newAuditGaugeVec(
		"audit_prev_root_per_server",
		"Previous root index used for the latest audit.",
	),
	AuditCurrRootPerServer: newAuditGaugeVec(
		"audit_curr_root_per_server",
		"Current root index used for the latest audit.",
	),
	AuditRunAtPerServer: newAuditGaugeVec(
		"audit_run_at_per_server",
		"Timestamp in unix seconds at which latest audit run.",
	),
}

// StartMetrics listens and servers the HTTP metrics server in a new goroutine.
// The server is then returned and can be stopped using Close().
func StartMetrics(
	addr string,
	l logger.Logger,
	uptimeCounter func() float64,
) *http.Server {
	Metrics.WithUptimeCounter(uptimeCounter)
	// expvar package adds a handler in to the default HTTP server (which has to be started explicitly),
	// and serves up the metrics at the /debug/vars endpoint.
	// Here we're registering both expvar and promhttp handlers in our custom server.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/debug/vars", expvar.Handler())
	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				l.Infof("Metrics http server closed")
			} else {
				l.Errorf("Metrics error: %s", err)
			}

		}
	}()

	return server
}
