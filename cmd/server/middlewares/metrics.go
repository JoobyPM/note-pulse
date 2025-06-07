// cmd/server/middlewares/metrics.go
package middlewares

import (
	"strconv"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// normalizeRoutePath returns the route template to prevent high cardinality
// in metrics labels. Returns the actual path for unmatched routes (404s).
func normalizeRoutePath(c *fiber.Ctx) string {
	if route := c.Route(); route != nil {
		return route.Path // already the template (e.g., "/notes/:id")
	}
	return c.Path() // fallback for 404 etc.
}

// normalizeStatus returns the status code as a string for Prometheus metrics
// 2xx -> "2xx", 4xx -> "4xx", 5xx -> "5xx"
func normalizeStatus(status int) string {
	if status >= 200 && status < 300 {
		return "2xx"
	} else if status >= 400 && status < 500 {
		return "4xx"
	} else if status >= 500 && status < 600 {
		return "5xx"
	}
	return strconv.Itoa(status)
}

// AttachMetrics gives the supplied Fiber app its **own** Prometheus registry
// and wires a /metrics endpoint plus request-timing middleware.
func AttachMetrics(app *fiber.App) {
	reg := prometheus.NewRegistry()

	// collectors
	reqDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	reqTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	reg.MustRegister(reqDuration, reqTotal)

	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		dur := time.Since(start).Seconds()

		method := c.Method()
		path := normalizeRoutePath(c)
		status := normalizeStatus(c.Response().StatusCode())

		reqDuration.WithLabelValues(method, path, status).Observe(dur)
		reqTotal.WithLabelValues(method, path, status).Inc()
		return err
	})

	// /metrics handler (uses *this* registry)
	app.Get("/metrics", adaptor.HTTPHandler(
		promhttp.HandlerFor(reg, promhttp.HandlerOpts{})),
	)
}
