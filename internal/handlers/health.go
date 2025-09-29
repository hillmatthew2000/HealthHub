package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthStatus struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Checks    map[string]interface{} `json:"checks,omitempty"`
}

// HealthCheck provides a basic health check endpoint
func HealthCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := HealthStatus{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0", // This could come from build-time variables
		}

		c.JSON(http.StatusOK, status)
	}
}

// LivenessCheck provides Kubernetes liveness probe endpoint
func LivenessCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		status := HealthStatus{
			Status:    "alive",
			Timestamp: time.Now(),
		}

		c.JSON(http.StatusOK, status)
	}
}

// ReadinessCheck provides Kubernetes readiness probe endpoint
func ReadinessCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		checks := make(map[string]interface{})
		overallStatus := "ready"

		// Check database connectivity
		sqlDB, err := db.DB()
		if err != nil {
			checks["database"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			overallStatus = "not ready"
		} else {
			if err := sqlDB.Ping(); err != nil {
				checks["database"] = map[string]interface{}{
					"status": "unhealthy",
					"error":  err.Error(),
				}
				overallStatus = "not ready"
			} else {
				// Get database stats
				stats := sqlDB.Stats()
				checks["database"] = map[string]interface{}{
					"status":           "healthy",
					"open_connections": stats.OpenConnections,
					"max_open_conns":   stats.MaxOpenConnections,
					"in_use":           stats.InUse,
					"idle":             stats.Idle,
				}
			}
		}

		// Check memory usage
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		checks["memory"] = map[string]interface{}{
			"alloc_mb":       bToMb(m.Alloc),
			"total_alloc_mb": bToMb(m.TotalAlloc),
			"sys_mb":         bToMb(m.Sys),
			"num_gc":         m.NumGC,
		}

		// Check goroutines
		checks["goroutines"] = runtime.NumGoroutine()

		status := HealthStatus{
			Status:    overallStatus,
			Timestamp: time.Now(),
			Checks:    checks,
		}

		httpStatus := http.StatusOK
		if overallStatus != "ready" {
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, status)
	}
}

// DetailedHealthCheck provides comprehensive health information
func DetailedHealthCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		checks := make(map[string]interface{})
		overallStatus := "healthy"

		// Database health check with connection pool info
		sqlDB, err := db.DB()
		if err != nil {
			checks["database"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			overallStatus = "degraded"
		} else {
			start := time.Now()
			if err := sqlDB.Ping(); err != nil {
				checks["database"] = map[string]interface{}{
					"status":        "unhealthy",
					"error":         err.Error(),
					"response_time": time.Since(start).Milliseconds(),
				}
				overallStatus = "degraded"
			} else {
				stats := sqlDB.Stats()
				checks["database"] = map[string]interface{}{
					"status":           "healthy",
					"response_time_ms": time.Since(start).Milliseconds(),
					"open_connections": stats.OpenConnections,
					"max_open_conns":   stats.MaxOpenConnections,
					"in_use":           stats.InUse,
					"idle":             stats.Idle,
					"wait_count":       stats.WaitCount,
					"wait_duration_ms": stats.WaitDuration.Milliseconds(),
				}
			}
		}

		// Memory and runtime information
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		checks["runtime"] = map[string]interface{}{
			"go_version": runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
			"cgo_calls":  runtime.NumCgoCall(),
			"memory": map[string]interface{}{
				"alloc_mb":        bToMb(m.Alloc),
				"total_alloc_mb":  bToMb(m.TotalAlloc),
				"sys_mb":          bToMb(m.Sys),
				"heap_alloc_mb":   bToMb(m.HeapAlloc),
				"heap_sys_mb":     bToMb(m.HeapSys),
				"heap_idle_mb":    bToMb(m.HeapIdle),
				"heap_inuse_mb":   bToMb(m.HeapInuse),
				"stack_inuse_mb":  bToMb(m.StackInuse),
				"stack_sys_mb":    bToMb(m.StackSys),
				"num_gc":          m.NumGC,
				"gc_cpu_fraction": m.GCCPUFraction,
			},
		}

		// System information
		checks["system"] = map[string]interface{}{
			"num_cpu":   runtime.NumCPU(),
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
			"max_procs": runtime.GOMAXPROCS(0),
		}

		// Uptime information
		checks["uptime"] = map[string]interface{}{
			"started_at":     time.Now().Add(-time.Since(time.Now())), // This would be set at startup
			"uptime_seconds": time.Since(time.Now()).Seconds(),        // This would be calculated from startup time
		}

		status := HealthStatus{
			Status:    overallStatus,
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Checks:    checks,
		}

		httpStatus := http.StatusOK
		if overallStatus == "degraded" {
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, status)
	}
}

// bToMb converts bytes to megabytes
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// CORS middleware
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// SecurityHeaders middleware adds security headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}
