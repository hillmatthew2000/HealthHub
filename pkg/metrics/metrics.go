package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Registry holds all the metrics for the application
type Registry struct {
	// HTTP metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpRequestSize     *prometheus.HistogramVec
	httpResponseSize    *prometheus.HistogramVec

	// Database metrics
	dbConnectionsTotal  prometheus.Gauge
	dbConnectionsActive prometheus.Gauge
	dbQueryDuration     *prometheus.HistogramVec
	dbTransactionsTotal *prometheus.CounterVec

	// Business metrics
	patientsTotal     prometheus.Gauge
	observationsTotal prometheus.Gauge
	authAttemptsTotal *prometheus.CounterVec
	authTokensActive  prometheus.Gauge

	// System metrics
	goroutinesActive prometheus.Gauge
	memoryUsage      prometheus.Gauge
	gcDuration       prometheus.Summary
}

// NewRegistry creates a new metrics registry with all application metrics
func NewRegistry() *Registry {
	r := &Registry{
		// HTTP Metrics
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),

		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds",
				Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
			},
			[]string{"method", "endpoint"},
		),

		httpRequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "Size of HTTP requests in bytes",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000},
			},
			[]string{"method", "endpoint"},
		),

		httpResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "Size of HTTP responses in bytes",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000},
			},
			[]string{"method", "endpoint"},
		),

		// Database Metrics
		dbConnectionsTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "database_connections_total",
				Help: "Total number of database connections",
			},
		),

		dbConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "database_connections_active",
				Help: "Number of active database connections",
			},
		),

		dbQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "database_query_duration_seconds",
				Help:    "Duration of database queries in seconds",
				Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.0, 5.0},
			},
			[]string{"operation", "table"},
		),

		dbTransactionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "database_transactions_total",
				Help: "Total number of database transactions",
			},
			[]string{"operation", "table", "status"},
		),

		// Business Metrics
		patientsTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "patients_total",
				Help: "Total number of patients in the system",
			},
		),

		observationsTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "observations_total",
				Help: "Total number of observations in the system",
			},
		),

		authAttemptsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_attempts_total",
				Help: "Total number of authentication attempts",
			},
			[]string{"method", "status"},
		),

		authTokensActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "auth_tokens_active",
				Help: "Number of active authentication tokens",
			},
		),

		// System Metrics
		goroutinesActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "goroutines_active",
				Help: "Number of active goroutines",
			},
		),

		memoryUsage: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "memory_usage_bytes",
				Help: "Current memory usage in bytes",
			},
		),

		gcDuration: promauto.NewSummary(
			prometheus.SummaryOpts{
				Name:       "gc_duration_seconds",
				Help:       "Duration of garbage collection cycles",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			},
		),
	}

	// Start background metrics collection
	go r.collectSystemMetrics()

	return r
}

// PrometheusMiddleware returns a Gin middleware that collects HTTP metrics
func (r *Registry) PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Calculate metrics
		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		endpoint := c.FullPath()

		// If endpoint is empty (404), use "unknown"
		if endpoint == "" {
			endpoint = "unknown"
		}

		// Record metrics
		r.httpRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
		r.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration)

		// Record request/response sizes if available
		if c.Request.ContentLength > 0 {
			r.httpRequestSize.WithLabelValues(method, endpoint).Observe(float64(c.Request.ContentLength))
		}

		responseSize := float64(c.Writer.Size())
		if responseSize > 0 {
			r.httpResponseSize.WithLabelValues(method, endpoint).Observe(responseSize)
		}
	}
}

// Database Metrics Methods
func (r *Registry) RecordDBConnection(total, active int) {
	r.dbConnectionsTotal.Set(float64(total))
	r.dbConnectionsActive.Set(float64(active))
}

func (r *Registry) RecordDBQuery(operation, table string, duration time.Duration) {
	r.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

func (r *Registry) RecordDBTransaction(operation, table, status string) {
	r.dbTransactionsTotal.WithLabelValues(operation, table, status).Inc()
}

// Business Metrics Methods
func (r *Registry) SetPatientsTotal(count int) {
	r.patientsTotal.Set(float64(count))
}

func (r *Registry) SetObservationsTotal(count int) {
	r.observationsTotal.Set(float64(count))
}

func (r *Registry) RecordAuthAttempt(method, status string) {
	r.authAttemptsTotal.WithLabelValues(method, status).Inc()
}

func (r *Registry) SetActiveTokens(count int) {
	r.authTokensActive.Set(float64(count))
}

// System Metrics Methods
func (r *Registry) SetGoroutines(count int) {
	r.goroutinesActive.Set(float64(count))
}

func (r *Registry) SetMemoryUsage(bytes uint64) {
	r.memoryUsage.Set(float64(bytes))
}

func (r *Registry) RecordGCDuration(duration time.Duration) {
	r.gcDuration.Observe(duration.Seconds())
}

// collectSystemMetrics runs in background to collect system-level metrics
func (r *Registry) collectSystemMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		r.collectRuntimeMetrics()
	}
}

func (r *Registry) collectRuntimeMetrics() {
	// This would collect runtime metrics like memory usage, goroutines, etc.
	// Implementation depends on specific requirements and available runtime APIs
}

// Custom metrics for specific use cases
type Timer struct {
	start  time.Time
	metric *prometheus.HistogramVec
	labels []string
}

func (r *Registry) StartTimer(metric *prometheus.HistogramVec, labels ...string) *Timer {
	return &Timer{
		start:  time.Now(),
		metric: metric,
		labels: labels,
	}
}

func (t *Timer) ObserveDuration() {
	duration := time.Since(t.start).Seconds()
	t.metric.WithLabelValues(t.labels...).Observe(duration)
}

// Health check metrics
func (r *Registry) GetHealthMetrics() map[string]interface{} {
	return map[string]interface{}{
		"goroutines_active":     r.goroutinesActive,
		"memory_usage":          r.memoryUsage,
		"db_connections_total":  r.dbConnectionsTotal,
		"db_connections_active": r.dbConnectionsActive,
	}
}
