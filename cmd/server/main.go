package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/config"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/pkg/database"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		panic("Configuration validation failed: " + err.Error())
	}

	// Initialize logger
	if cfg.IsDevelopment() {
		logger.InitDevelopment()
	} else {
		logger.Init(cfg.LogLevel)
	}
	defer logger.Sync()

	logger.Info("Starting HealthHub API",
		zap.String("version", "1.0.0"),
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Port),
	)

	// Initialize database
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Run database migrations
	if err := database.AutoMigrate(db); err != nil {
		logger.Fatal("Failed to migrate database", zap.Error(err))
	}

	// Create database indexes
	if err := database.CreateIndexes(db); err != nil {
		logger.Warn("Failed to create some database indexes", zap.Error(err))
	}

	// Initialize RBAC service and create default roles
	rbacService := auth.NewRBACService(db)
	if err := rbacService.InitializeDefaultRoles(); err != nil {
		logger.Warn("Failed to initialize default roles", zap.Error(err))
	}

	// Initialize Gin router
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Add middleware
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			logger.LogHTTPRequest(
				param.Method,
				param.Path,
				param.StatusCode,
				param.Latency.Milliseconds(),
				param.Keys["user_id"].(string),
			)
			return ""
		},
	}))
	r.Use(gin.Recovery())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowed := false

		for _, allowedOrigin := range cfg.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Security headers middleware
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})

	// Health check endpoint
	r.GET(cfg.HealthCheckPath, func(c *gin.Context) {
		// Check database connectivity
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(500, handlers.HealthResponse{
				Status:    "unhealthy",
				Timestamp: time.Now(),
				Services: map[string]string{
					"database": "error: " + err.Error(),
				},
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(500, handlers.HealthResponse{
				Status:    "unhealthy",
				Timestamp: time.Now(),
				Services: map[string]string{
					"database": "error: " + err.Error(),
				},
			})
			return
		}

		c.JSON(200, handlers.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Services: map[string]string{
				"database": "ok",
				"api":      "ok",
			},
		})
	})

	// Initialize token manager
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, "HealthHub API")

	// Initialize handlers
	patientHandler := handlers.NewPatientHandler(db)
	observationHandler := handlers.NewObservationHandler(db)
	authHandler := handlers.NewAuthHandler(db, cfg.JWTSecret)

	// Public routes
	public := r.Group("/api/v1")
	{
		public.POST("/auth/login", authHandler.Login)
		public.POST("/auth/register", authHandler.Register)
	}

	// Protected routes
	protected := r.Group("/api/v1")
	protected.Use(auth.AuthMiddleware(tokenManager))
	{
		// Auth routes
		auth := protected.Group("/auth")
		{
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.GET("/profile", authHandler.GetProfile)
			auth.POST("/change-password", authHandler.ChangePassword)
		}

		// Patient endpoints
		patients := protected.Group("/patients")
		{
			patients.POST("", auth.RequireRole("practitioner", "admin"), patientHandler.CreatePatient)
			patients.GET("", auth.RequireRole("practitioner", "admin", "nurse"), patientHandler.GetPatients)
			patients.GET("/:id", auth.RequireRole("practitioner", "admin", "nurse"), patientHandler.GetPatient)
			patients.PUT("/:id", auth.RequireRole("practitioner", "admin"), patientHandler.UpdatePatient)
			patients.DELETE("/:id", auth.RequireRole("admin"), patientHandler.DeletePatient)
			patients.GET("/:patientId/observations", auth.RequireRole("practitioner", "admin", "nurse"), observationHandler.GetPatientObservations)
		}

		// Observation endpoints
		observations := protected.Group("/observations")
		{
			observations.POST("", auth.RequireRole("practitioner", "admin", "lab-tech"), observationHandler.CreateObservation)
			observations.GET("", auth.RequireRole("practitioner", "admin", "nurse"), observationHandler.GetObservations)
			observations.GET("/:id", auth.RequireRole("practitioner", "admin", "nurse"), observationHandler.GetObservation)
			observations.PUT("/:id", auth.RequireRole("practitioner", "admin"), observationHandler.UpdateObservation)
			observations.DELETE("/:id", auth.RequireRole("admin"), observationHandler.DeleteObservation)
		}
	}

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Server starting", zap.String("addr", srv.Addr))

		if cfg.TLSEnabled {
			if err := srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil && err != http.ErrServerClosed {
				logger.Fatal("Failed to start HTTPS server", zap.Error(err))
			}
		} else {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Fatal("Failed to start HTTP server", zap.Error(err))
			}
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Server shutting down...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	// Close database connection
	if err := database.CloseDB(db); err != nil {
		logger.Error("Failed to close database connection", zap.Error(err))
	}

	logger.Info("Server exited")
}
