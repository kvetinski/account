package telemetry

import (
	"database/sql"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	grpcRequestsTotal    *prometheus.CounterVec
	grpcRequestDuration  *prometheus.HistogramVec
	grpcRequestsInFlight prometheus.Gauge

	dbQueriesTotal  *prometheus.CounterVec
	dbQueryDuration *prometheus.HistogramVec
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	m := &Metrics{
		grpcRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "account_grpc_requests_total",
				Help: "Total gRPC requests by method and code.",
			},
			[]string{"method", "code"},
		),
		grpcRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "account_grpc_request_duration_seconds",
				Help:    "gRPC request latency in seconds by method and code.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "code"},
		),
		grpcRequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "account_grpc_requests_in_flight",
				Help: "Current number of in-flight gRPC requests.",
			},
		),
		dbQueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "account_db_queries_total",
				Help: "Total DB method calls by method and status.",
			},
			[]string{"method", "status"},
		),
		dbQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "account_db_query_duration_seconds",
				Help:    "DB method duration in seconds by method and status.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "status"},
		),
	}

	registerer.MustRegister(
		m.grpcRequestsTotal,
		m.grpcRequestDuration,
		m.grpcRequestsInFlight,
		m.dbQueriesTotal,
		m.dbQueryDuration,
	)

	return m
}

func (m *Metrics) ObserveRPC(method, code string, duration time.Duration) {
	if m == nil {
		return
	}

	m.grpcRequestsTotal.WithLabelValues(method, code).Inc()
	m.grpcRequestDuration.WithLabelValues(method, code).Observe(duration.Seconds())
}

func (m *Metrics) IncRPCInFlight() {
	if m == nil {
		return
	}

	m.grpcRequestsInFlight.Inc()
}

func (m *Metrics) DecRPCInFlight() {
	if m == nil {
		return
	}

	m.grpcRequestsInFlight.Dec()
}

func (m *Metrics) ObserveDB(method, status string, duration time.Duration) {
	if m == nil {
		return
	}

	m.dbQueriesTotal.WithLabelValues(method, status).Inc()
	m.dbQueryDuration.WithLabelValues(method, status).Observe(duration.Seconds())
}

func RegisterDBPoolMetrics(db *sql.DB, registerer prometheus.Registerer) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	collectors := []prometheus.Collector{
		prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "account_db_pool_open_connections",
				Help: "Open database connections.",
			},
			func() float64 { return float64(db.Stats().OpenConnections) },
		),
		prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "account_db_pool_in_use_connections",
				Help: "In-use database connections.",
			},
			func() float64 { return float64(db.Stats().InUse) },
		),
		prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "account_db_pool_idle_connections",
				Help: "Idle database connections.",
			},
			func() float64 { return float64(db.Stats().Idle) },
		),
		prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Name: "account_db_pool_wait_count_total",
				Help: "Total number of waits for a free connection.",
			},
			func() float64 { return float64(db.Stats().WaitCount) },
		),
		prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Name: "account_db_pool_wait_duration_seconds_total",
				Help: "Total time blocked waiting for a free connection in seconds.",
			},
			func() float64 { return db.Stats().WaitDuration.Seconds() },
		),
		prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Name: "account_db_pool_max_idle_closed_total",
				Help: "Total connections closed due to MaxIdleConns.",
			},
			func() float64 { return float64(db.Stats().MaxIdleClosed) },
		),
		prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Name: "account_db_pool_max_lifetime_closed_total",
				Help: "Total connections closed due to ConnMaxLifetime.",
			},
			func() float64 { return float64(db.Stats().MaxLifetimeClosed) },
		),
	}

	for _, c := range collectors {
		if err := registerer.Register(c); err != nil {
			var already prometheus.AlreadyRegisteredError
			if errors.As(err, &already) {
				continue
			}
			return err
		}
	}

	return nil
}
