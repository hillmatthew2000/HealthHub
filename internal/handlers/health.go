package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// Pinger checks a dependency. *pgxpool.Pool implements it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthStatus is the body of the health probes.
type HealthStatus struct {
	Status  string            `json:"status"`
	Version string            `json:"version,omitempty"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// readinessTimeout bounds the dependency checks so a hung database fails
// the probe instead of hanging it.
const readinessTimeout = 2 * time.Second

// LivenessCheck reports that the process is serving requests. It never
// touches dependencies, so a database outage does not restart every pod.
func LivenessCheck(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, HealthStatus{Status: "alive", Version: version})
	}
}

// ReadinessCheck reports whether the service can handle traffic, which
// requires a reachable database.
func ReadinessCheck(db Pinger, version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
		defer cancel()

		status := HealthStatus{Status: "ready", Version: version, Checks: map[string]string{"database": "up"}}
		code := http.StatusOK
		if err := db.Ping(ctx); err != nil {
			// Details go to the log; the probe is unauthenticated.
			logger.Warn("readiness check failed", zap.Error(err))
			status.Status, status.Checks["database"] = "not ready", "down"
			code = http.StatusServiceUnavailable
		}
		c.JSON(code, status)
	}
}
