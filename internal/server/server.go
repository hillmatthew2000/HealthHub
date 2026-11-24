// Package server assembles the HTTP router: middleware, routes and the
// access policy for each route.
package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/internal/middleware"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// Options configures the router.
type Options struct {
	Version           string
	HSTS              bool
	TrustedProxies    []string
	AllowedOrigins    []string
	MaxBodyBytes      int64
	RequestTimeout    time.Duration
	Paging            handlers.Paging
	AllowRegistration bool
}

// Tokens issues and verifies access tokens.
type Tokens interface {
	auth.TokenVerifier
	handlers.TokenIssuer
}

// Patients is the patient repository.
type Patients interface {
	handlers.PatientStore
	handlers.PatientLookup
}

// Deps are the router's collaborators.
type Deps struct {
	DB           handlers.Pinger
	Users        handlers.UserStore
	Patients     Patients
	Observations handlers.ObservationStore
	Tokens       Tokens
	Passwords    *auth.PasswordHasher
	// Audit records access to PHI and account changes. It defaults to
	// handlers.LogAuditor.
	Audit handlers.Auditor
	// AuditLog serves the audit trail to administrators. The audit-events
	// route is only registered when it is set.
	AuditLog handlers.AuditEventStore
}

// New returns the API router.
func New(opts Options, deps Deps) (*gin.Engine, error) {
	if deps.Audit == nil {
		deps.Audit = handlers.LogAuditor{}
	}
	r := gin.New()
	// Only trust X-Forwarded-For from configured proxies; otherwise any
	// client could spoof its IP address.
	if err := r.SetTrustedProxies(opts.TrustedProxies); err != nil {
		return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}
	r.HandleMethodNotAllowed = true

	r.Use(
		middleware.RequestID(),
		requestLogger(),
		gin.CustomRecovery(func(c *gin.Context, _ any) { problem.Write(c, problem.Internal()) }),
		middleware.SecurityHeaders(opts.HSTS),
		middleware.CORS(opts.AllowedOrigins),
	)
	if opts.MaxBodyBytes > 0 {
		r.Use(middleware.MaxBodyBytes(opts.MaxBodyBytes))
	}
	if opts.RequestTimeout > 0 {
		r.Use(middleware.Timeout(opts.RequestTimeout))
	}
	r.NoRoute(func(c *gin.Context) {
		problem.Write(c, problem.NotFound("No route matches "+c.Request.Method+" "+c.Request.URL.Path+"."))
	})
	r.NoMethod(func(c *gin.Context) {
		problem.Write(c, problem.New(http.StatusMethodNotAllowed, problem.CodeMethodNotAllowed,
			c.Request.Method+" is not supported on "+c.Request.URL.Path+"."))
	})

	// Health probes: liveness never touches dependencies, readiness checks
	// the database. /health is kept as an alias of readiness.
	r.GET("/health/live", handlers.LivenessCheck(opts.Version))
	r.GET("/health/ready", handlers.ReadinessCheck(deps.DB, opts.Version))
	r.GET("/health", handlers.ReadinessCheck(deps.DB, opts.Version))

	authHandler := handlers.NewAuthHandler(deps.Users, deps.Tokens, deps.Passwords, opts.AllowRegistration)
	patientHandler := handlers.NewPatientHandler(deps.Patients, deps.Audit, opts.Paging)
	observationHandler := handlers.NewObservationHandler(deps.Observations, deps.Patients, deps.Audit, opts.Paging)
	userHandler := handlers.NewUserHandler(deps.Users, deps.Passwords, deps.Audit, opts.Paging)

	api := r.Group("/api/v1")
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/register", authHandler.Register)

	protected := api.Group("")
	protected.Use(auth.Authenticate(deps.Tokens))

	authRoutes := protected.Group("/auth")
	authRoutes.POST("/refresh", authHandler.RefreshToken)
	authRoutes.GET("/profile", authHandler.GetProfile)
	authRoutes.POST("/change-password", authHandler.ChangePassword)

	can := auth.Require
	patients := protected.Group("/patients")
	patients.POST("", can(auth.ResourcePatients, auth.ActionCreate), patientHandler.CreatePatient)
	patients.GET("", can(auth.ResourcePatients, auth.ActionRead), patientHandler.GetPatients)
	patients.GET("/:id", can(auth.ResourcePatients, auth.ActionRead), patientHandler.GetPatient)
	patients.PUT("/:id", can(auth.ResourcePatients, auth.ActionUpdate), patientHandler.UpdatePatient)
	patients.DELETE("/:id", can(auth.ResourcePatients, auth.ActionDelete), patientHandler.DeletePatient)
	patients.GET("/:id/observations", can(auth.ResourceObservations, auth.ActionRead), observationHandler.GetPatientObservations)

	observations := protected.Group("/observations")
	observations.POST("", can(auth.ResourceObservations, auth.ActionCreate), observationHandler.CreateObservation)
	observations.GET("", can(auth.ResourceObservations, auth.ActionRead), observationHandler.GetObservations)
	observations.GET("/:id", can(auth.ResourceObservations, auth.ActionRead), observationHandler.GetObservation)
	observations.PUT("/:id", can(auth.ResourceObservations, auth.ActionUpdate), observationHandler.UpdateObservation)
	observations.DELETE("/:id", can(auth.ResourceObservations, auth.ActionDelete), observationHandler.DeleteObservation)

	users := protected.Group("/users")
	users.POST("", can(auth.ResourceUsers, auth.ActionCreate), userHandler.CreateUser)
	users.GET("", can(auth.ResourceUsers, auth.ActionRead), userHandler.GetUsers)
	users.GET("/:id", can(auth.ResourceUsers, auth.ActionRead), userHandler.GetUser)
	users.PATCH("/:id", can(auth.ResourceUsers, auth.ActionUpdate), userHandler.UpdateUser)

	if deps.AuditLog != nil {
		auditHandler := handlers.NewAuditHandler(deps.AuditLog, opts.Paging)
		protected.GET("/audit-events", can(auth.ResourceAuditLog, auth.ActionRead), auditHandler.GetAuditEvents)
	}

	return r, nil
}

// requestLogger logs one line per request. It logs the route template
// rather than the raw URL: paths and query strings can contain identifiers
// and search terms that are PHI.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		logger.LogHTTPRequest(c.Request.Method, route, c.Writer.Status(),
			time.Since(start).Milliseconds(), c.GetString(auth.UserIDKey), c.GetString(problem.RequestIDKey))
	}
}
