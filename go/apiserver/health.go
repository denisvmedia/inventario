package apiserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/registry"
)

const defaultHealthCheckTimeout = 5 * time.Second

// RedisPinger is an optional Redis dependency for readiness checks.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

type healthAPI struct {
	factorySet *registry.FactorySet
	redis      RedisPinger
	timeout    time.Duration
}

type livenessResponse struct {
	Status string `json:"status"`
}

type readinessCheck struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

type readinessResponse struct {
	Status    string                    `json:"status"`
	Timestamp string                    `json:"timestamp"`
	Checks    map[string]readinessCheck `json:"checks"`
}

func (api *healthAPI) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(livenessResponse{
		Status: "alive",
	})
}

func (api *healthAPI) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), api.timeout)
	defer cancel()

	resp := readinessResponse{
		Status:    "ready",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    make(map[string]readinessCheck),
	}
	httpStatus := http.StatusOK

	dbStartedAt := time.Now()
	if err := api.factorySet.Ping(ctx); err != nil {
		httpStatus = http.StatusServiceUnavailable
		resp.Status = "not_ready"
		resp.Checks["database"] = readinessCheck{
			Status: "error",
			Error:  err.Error(),
		}
	} else {
		resp.Checks["database"] = readinessCheck{
			Status:  "ok",
			Latency: time.Since(dbStartedAt).String(),
		}
	}

	if api.redis == nil {
		resp.Checks["redis"] = readinessCheck{Status: "skipped"}
	} else {
		redisStartedAt := time.Now()
		if err := api.redis.Ping(ctx); err != nil {
			httpStatus = http.StatusServiceUnavailable
			resp.Status = "not_ready"
			resp.Checks["redis"] = readinessCheck{
				Status: "error",
				Error:  err.Error(),
			}
		} else {
			resp.Checks["redis"] = readinessCheck{
				Status:  "ok",
				Latency: time.Since(redisStartedAt).String(),
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(resp)
}

// Health mounts infrastructure health check endpoints at the router root.
func Health(factorySet *registry.FactorySet, redisPinger RedisPinger) func(r chi.Router) {
	api := &healthAPI{
		factorySet: factorySet,
		redis:      redisPinger,
		timeout:    defaultHealthCheckTimeout,
	}

	return func(r chi.Router) {
		r.Get("/healthz", api.healthz)
		r.Get("/readyz", api.readyz)
	}
}
