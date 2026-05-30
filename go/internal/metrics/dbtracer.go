package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// queryTraceKey is the unexported context key under which the query
// start time and parsed verb are stashed between TraceQueryStart and
// TraceQueryEnd. A dedicated unexported type guarantees no collision
// with other context values.
type queryTraceKey struct{}

// queryTraceData carries per-query state across the trace lifecycle.
type queryTraceData struct {
	start time.Time
	verb  string
}

// QueryTracer implements pgx.QueryTracer to record per-query latency
// and counts into the Prometheus default registry. The SQL operation
// (select/insert/update/...) is the only label, keeping cardinality
// bounded — see parseSQLVerb.
type QueryTracer struct{}

// compile-time assertion that QueryTracer satisfies pgx.QueryTracer.
var _ pgx.QueryTracer = (*QueryTracer)(nil)

// NewQueryTracer constructs a QueryTracer ready to be attached to a
// pgx (pool) config via the QueryTracer field.
func NewQueryTracer() *QueryTracer {
	return &QueryTracer{}
}

// TraceQueryStart stashes the start time and parsed SQL verb in the
// returned context for retrieval in TraceQueryEnd.
func (*QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTraceKey{}, queryTraceData{
		start: time.Now(),
		verb:  parseSQLVerb(data.SQL),
	})
}

// TraceQueryEnd observes the elapsed duration and increments the query
// counter, partitioned by operation and outcome. If the start context
// value is missing (which should not happen), it no-ops safely.
func (*QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	td, ok := ctx.Value(queryTraceKey{}).(queryTraceData)
	if !ok {
		return
	}

	status := "ok"
	if data.Err != nil {
		status = "error"
	}

	dbQueryDuration.WithLabelValues(td.verb).Observe(time.Since(td.start).Seconds())
	dbQueriesTotal.WithLabelValues(td.verb, status).Inc()
}
