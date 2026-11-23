// Command server runs the HealthHub API.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/config"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/internal/middleware"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/pkg/database"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
	"go.uber.org/zap"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "healthhub:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.IsDevelopment() {
		logger.InitDevelopment()
	} else {
		logger.Init(cfg.LogLevel)
	}
	defer logger.Sync()

	logger.Info("starting HealthHub API",
		zap.String("version", version),
		zap.String("environment", cfg.Environment),
		zap.String("addr", cfg.Addr()),
	)

	tokens, err := auth.NewTokenManager(auth.TokenConfig{
		Secret:   []byte(cfg.JWTSecret),
		Issuer:   cfg.JWTIssuer,
		Audience: cfg.JWTAudience,
		TTL:      cfg.JWTTTL,
	})
	if err != nil {
		return err
	}
	passwords, err := auth.NewPasswordHasher(cfg.BcryptCost)
	if err != nil {
		return err
	}

	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := database.CloseDB(db); err != nil {
			logger.Error("failed to close database connection", zap.Error(err))
		}
	}()
	if err := database.AutoMigrate(db); err != nil {
		return err
	}
	if err := database.CreateIndexes(db); err != nil {
		logger.Warn("failed to create some database indexes", zap.Error(err))
	}
	if err := auth.NewRBACService(db).InitializeDefaultRoles(); err != nil {
		logger.Warn("failed to initialize default roles", zap.Error(err))
	}

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	// Only trust X-Forwarded-For from configured proxies; otherwise any
	// client could spoof its IP address.
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		// Log the route template rather than the raw URL: paths and query
		// strings can contain identifiers and search terms that are PHI.
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		logger.LogHTTPRequest(c.Request.Method, route, c.Writer.Status(),
			time.Since(start).Milliseconds(), c.GetString(auth.UserIDKey))
	})
	r.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		problem.Write(c, problem.Internal())
	}))
	r.Use(middleware.SecurityHeaders(cfg.TLSEnabled || cfg.IsProduction()))
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxBodyBytes)
		c.Next()
	})
	r.NoRoute(func(c *gin.Context) {
		problem.Write(c, problem.NotFound("No route matches "+c.Request.Method+" "+c.Request.URL.Path+"."))
	})

	// Health probes: liveness never touches dependencies, readiness checks
	// the database. /health is kept as an alias of readiness.
	r.GET("/health/live", handlers.LivenessCheck())
	r.GET("/health/ready", handlers.ReadinessCheck(db))
	r.GET("/health", handlers.ReadinessCheck(db))

	patientHandler := handlers.NewPatientHandler(db)
	observationHandler := handlers.NewObservationHandler(db)
	authHandler := handlers.NewAuthHandler(db, tokens, passwords, cfg.AllowRegistration)

	api := r.Group("/api/v1")
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/register", authHandler.Register)

	protected := api.Group("")
	protected.Use(auth.Authenticate(tokens))

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

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", zap.String("addr", srv.Addr), zap.Bool("tls", cfg.TLSEnabled))
		if cfg.TLSEnabled {
			serveErr <- srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			serveErr <- srv.ListenAndServe()
		}
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	logger.Info("shutting down", zap.Duration("timeout", cfg.ShutdownTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}
	logger.Info("server stopped")
	return nil
}
