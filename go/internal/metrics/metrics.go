// Package metrics is the central Prometheus metric registry for
// Inventario's cross-cutting instrumentation (HTTP, database, auth,
// email, rate-limiting, and system-wide business counts).
//
// Every metric is registered with promauto against the default
// registry, which is the same registry promhttp.Handler() exposes on
// /metrics. This package is deliberately a leaf: it imports only the
// Prometheus client, pgx/v5 (+pgxpool), chi/v5, and the standard
// library. It MUST NOT import apiserver, services, registry, or
// models — wiring code in those packages calls the thin recording
// helpers exported here instead of touching the metric vars directly.
//
// Naming follows Prometheus best practice: every metric carries the
// inventario_ prefix; counters end in _total; durations are
// _seconds histograms; sizes are _bytes; gauges never carry _total.
//
// See issue #843 for the original instrumentation plan.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP request/response (RED) metrics. See HTTPMiddleware.
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_http_requests_total",
		Help: "Total number of HTTP requests handled, partitioned by method, chi route pattern, and status class.",
	}, []string{"method", "route", "status_class"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "inventario_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, partitioned by method and chi route pattern.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	}, []string{"method", "route"})

	httpRequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_http_requests_in_flight",
		Help: "Number of HTTP requests currently being served.",
	})
)

// Database query metrics. See QueryTracer.
var (
	dbQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "inventario_db_query_duration_seconds",
		Help:    "Database query latency in seconds, partitioned by SQL operation.",
		Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"operation"})

	dbQueriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_db_queries_total",
		Help: "Total number of database queries executed, partitioned by SQL operation and outcome.",
	}, []string{"operation", "status"})
)

// Authentication metrics.
var (
	authLoginAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_auth_login_attempts_total",
		Help: "Total number of tenant-plane login attempts, partitioned by outcome and authentication method (the back-office auth plane is not counted here).",
	}, []string{"outcome", "method"})

	authTokensIssuedTotal = newTokensIssuedTotal()
)

// Email processing metrics.
var (
	emailsProcessedTotal = newEmailsProcessedTotal()

	emailSendDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "inventario_email_send_duration_seconds",
		Help:    "Latency in seconds of a single email send attempt to the upstream provider.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	})

	emailQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_email_queue_depth",
		Help: "Number of emails currently waiting in the outbound queue.",
	})
)

// Rate-limiting metrics.
var rateLimitRejectionsTotal = newRateLimitRejectionsTotal()

// System-wide business gauges. Populated by BusinessCollector. These
// are deliberately unlabelled by tenant: they describe the whole
// installation, not a single tenant, so a high-cardinality tenant
// label would be both expensive and misleading.
var (
	businessTenants = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_tenants",
		Help: "Current number of tenants.",
	})
	businessUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_users",
		Help: "Current number of users.",
	})
	businessLocationGroups = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_location_groups",
		Help: "Current number of location groups.",
	})
	businessLocations = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_locations",
		Help: "Current number of locations.",
	})
	businessAreas = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_areas",
		Help: "Current number of areas.",
	})
	businessCommodities = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_commodities",
		Help: "Current number of commodities.",
	})
	businessFiles = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_files",
		Help: "Current number of files.",
	})

	businessFileStorageBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "inventario_file_storage_bytes",
		Help: "Current total file storage in bytes, partitioned by category.",
	}, []string{"category"})

	businessCollectDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "inventario_business_collect_duration_seconds",
		Help: "Latency in seconds of a single business-stats collection sweep.",
	})

	businessCollectErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_business_collect_errors_total",
		Help: "Total number of business-stats collection sweeps that failed.",
	})
)

// Token type label values for RecordTokenIssued.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Rate-limit scope label values for RecordRateLimitRejection.
const (
	RateLimitScopeGlobal = "global"
	RateLimitScopeAuth   = "auth"
)

// Email status label values for RecordEmailProcessed.
const (
	EmailStatusSent    = "sent"
	EmailStatusFailed  = "failed"
	EmailStatusRetried = "retried"
)

// File storage category label values for the file_storage_bytes gauge.
const (
	storageCategoryImages    = "images"
	storageCategoryDocuments = "documents"
	storageCategoryOther     = "other"
	storageCategoryExports   = "exports"
)

// The constructors below build a CounterVec and pre-initialise its
// bounded label series to 0, so the metric exports a zero-valued
// series before the first real event. That gives rate() a baseline
// and dashboards a non-empty series on a fresh deploy. They are
// var-initializer functions rather than an init() so the package
// keeps `gochecknoinits` happy (mirrors models/user.go).
//
// Counters whose label set depends on an enum this leaf package does
// not import (login outcomes/methods) are left lazy on purpose.

func newTokensIssuedTotal() *prometheus.CounterVec {
	cv := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_auth_tokens_issued_total",
		Help: "Total number of tenant-plane authentication tokens issued, partitioned by token type (the back-office auth plane is not counted here).",
	}, []string{"type"})
	cv.WithLabelValues(TokenTypeAccess).Add(0)
	cv.WithLabelValues(TokenTypeRefresh).Add(0)
	return cv
}

func newRateLimitRejectionsTotal() *prometheus.CounterVec {
	cv := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_rate_limit_rejections_total",
		Help: "Total number of requests rejected by a rate limiter, partitioned by scope.",
	}, []string{"scope"})
	cv.WithLabelValues(RateLimitScopeGlobal).Add(0)
	cv.WithLabelValues(RateLimitScopeAuth).Add(0)
	return cv
}

func newEmailsProcessedTotal() *prometheus.CounterVec {
	cv := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_emails_processed_total",
		Help: "Total number of email send outcomes, partitioned by status. 'sent' and 'failed' are terminal; 'retried' counts each requeue, so a message may contribute several 'retried' before one terminal outcome.",
	}, []string{"status"})
	cv.WithLabelValues(EmailStatusSent).Add(0)
	cv.WithLabelValues(EmailStatusFailed).Add(0)
	cv.WithLabelValues(EmailStatusRetried).Add(0)
	return cv
}

// RecordLoginAttempt increments the login-attempt counter for the
// given outcome (e.g. "success", "failure") and authentication method
// (e.g. "password", "mfa"). The caller owns the label vocabulary.
func RecordLoginAttempt(outcome, method string) {
	authLoginAttemptsTotal.WithLabelValues(outcome, method).Inc()
}

// RecordTokenIssued increments the issued-token counter for the given
// token type. Use the TokenType* constants.
func RecordTokenIssued(tokenType string) {
	authTokensIssuedTotal.WithLabelValues(tokenType).Inc()
}

// RecordRateLimitRejection increments the rate-limit rejection counter
// for the given scope. Use the RateLimitScope* constants.
func RecordRateLimitRejection(scope string) {
	rateLimitRejectionsTotal.WithLabelValues(scope).Inc()
}

// RecordEmailProcessed increments the processed-email counter for the
// given terminal status. Use the EmailStatus* constants.
func RecordEmailProcessed(status string) {
	emailsProcessedTotal.WithLabelValues(status).Inc()
}

// ObserveEmailSend records the duration of a single email send attempt.
func ObserveEmailSend(d time.Duration) {
	emailSendDuration.Observe(d.Seconds())
}

// SetEmailQueueDepth sets the current outbound email queue depth.
func SetEmailQueueDepth(n int) {
	emailQueueDepth.Set(float64(n))
}
