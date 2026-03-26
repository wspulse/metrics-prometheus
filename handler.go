package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler returns an http.Handler that serves the Prometheus /metrics endpoint.
// If a custom gatherer was provided via WithGatherer, it serves from that
// gatherer; otherwise it uses prometheus.DefaultGatherer.
func (c *Collector) Handler() http.Handler {
	return promhttp.HandlerFor(c.cfg.gatherer, promhttp.HandlerOpts{})
}
