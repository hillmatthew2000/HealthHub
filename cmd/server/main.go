// Command server runs the HealthHub API.
//
// Usage:
//
//	server [serve]   run the API (the default)
//	server migrate   apply pending database migrations and exit
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/config"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/internal/server"
	"github.com/hillmatthew2000/HealthHub/internal/store"
	"github.com/hillmatthew2000/HealthHub/pkg/encryption"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "healthhub:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "serve":
		return serve()
	case "migrate":
		return migrate(stdout)
	case "version":
		_, err := fmt.Fprintln(stdout, version)
		return err
	case "help", "-h", "-help", "--help":
		_, err := fmt.Fprint(stdout, usage)
		return err
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usage)
	}
}

const usage = `Usage: server <command>

Commands:
  serve     run the API (default)
  migrate   apply pending database migrations and exit
  version   print the version
`

// setup loads the configuration and initialises logging.
func setup() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.IsDevelopment() {
		logger.InitDevelopment()
	} else {
		logger.Init(cfg.LogLevel)
	}
	return cfg, nil
}

func openDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	return store.Open(ctx, store.PoolConfig{
		URL:             cfg.DatabaseURL,
		MaxConns:        cfg.DBMaxConns,
		MinConns:        cfg.DBMinConns,
		MaxConnLifetime: cfg.DBConnMaxLifetime,
		MaxConnIdleTime: cfg.DBConnMaxIdleTime,
	})
}

func applyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	applied, err := store.Migrate(ctx, db)
	if err != nil {
		return err
	}
	for _, v := range applied {
		logger.Info("applied database migration", zap.Int64("version", v))
	}
	return nil
}

func migrate(stdout io.Writer) error {
	cfg, err := setup()
	if err != nil {
		return err
	}
	defer logger.Sync()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	applied, err := store.Migrate(ctx, db)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "applied %d migration(s)\n", len(applied))
	return err
}

func serve() error {
	cfg, err := setup()
	if err != nil {
		return err
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
	master, err := encryption.ParseKey(cfg.EncryptionKey)
	if err != nil {
		return err
	}
	keys, err := encryption.NewKeys(master)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancelStart := context.WithTimeout(ctx, time.Minute)
	defer cancelStart()
	db, err := openDB(startCtx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if cfg.AutoMigrate {
		if err := applyMigrations(startCtx, db); err != nil {
			return err
		}
	}

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	patients := store.NewPatients(db, keys)
	router, err := server.New(server.Options{
		Version:           version,
		HSTS:              cfg.TLSEnabled || cfg.IsProduction(),
		TrustedProxies:    cfg.TrustedProxies,
		AllowedOrigins:    cfg.AllowedOrigins,
		MaxBodyBytes:      cfg.MaxBodyBytes,
		RequestTimeout:    cfg.RequestTimeout,
		Paging:            handlers.Paging{DefaultLimit: cfg.DefaultPageSize, MaxLimit: cfg.MaxPageSize},
		AllowRegistration: cfg.AllowRegistration,
	}, server.Deps{
		DB:           db,
		Users:        store.NewUsers(db),
		Patients:     patients,
		Observations: store.NewObservations(db, keys),
		Tokens:       tokens,
		Passwords:    passwords,
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

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
