package database

import (
	"fmt"
	"time"

	"github.com/hillmatthew2000/HealthHub/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	TimeZone string
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(databaseURL string) (*gorm.DB, error) {
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(postgres.Open(databaseURL), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool for security and performance
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// NewPostgresDBFromConfig creates a new PostgreSQL database connection from config
func NewPostgresDBFromConfig(config Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		config.Host, config.User, config.Password, config.DBName, config.Port, config.SSLMode, config.TimeZone)

	return NewPostgresDB(dsn)
}

// AutoMigrate runs database migrations for all models
func AutoMigrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Permission{},
		&models.UserRole{},
		&models.RolePermission{},
		&models.Patient{},
		&models.Observation{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

// CreateIndexes creates additional database indexes for performance
func CreateIndexes(db *gorm.DB) error {
	// User indexes
	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email ON users (email)").Error; err != nil {
		return fmt.Errorf("failed to create users email index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_active ON users (active)").Error; err != nil {
		return fmt.Errorf("failed to create users active index: %w", err)
	}

	// Patient indexes
	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_patients_active ON patients (active)").Error; err != nil {
		return fmt.Errorf("failed to create patients active index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_patients_created_at ON patients (created_at)").Error; err != nil {
		return fmt.Errorf("failed to create patients created_at index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_patients_created_by ON patients (created_by)").Error; err != nil {
		return fmt.Errorf("failed to create patients created_by index: %w", err)
	}

	// Use GIN index for JSON fields
	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_patients_name_gin ON patients USING GIN (name)").Error; err != nil {
		return fmt.Errorf("failed to create patients name gin index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_patients_telecom_gin ON patients USING GIN (telecom)").Error; err != nil {
		return fmt.Errorf("failed to create patients telecom gin index: %w", err)
	}

	// Observation indexes
	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_observations_status ON observations (status)").Error; err != nil {
		return fmt.Errorf("failed to create observations status index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_observations_effective_date ON observations (effective_date_time)").Error; err != nil {
		return fmt.Errorf("failed to create observations effective_date index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_observations_created_at ON observations (created_at)").Error; err != nil {
		return fmt.Errorf("failed to create observations created_at index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_observations_created_by ON observations (created_by)").Error; err != nil {
		return fmt.Errorf("failed to create observations created_by index: %w", err)
	}

	// GIN indexes for JSON fields in observations
	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_observations_subject_gin ON observations USING GIN (subject)").Error; err != nil {
		return fmt.Errorf("failed to create observations subject gin index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_observations_code_gin ON observations USING GIN (code)").Error; err != nil {
		return fmt.Errorf("failed to create observations code gin index: %w", err)
	}

	if err := db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_observations_category_gin ON observations USING GIN (category)").Error; err != nil {
		return fmt.Errorf("failed to create observations category gin index: %w", err)
	}

	return nil
}

// SetupSecurity configures database security settings
func SetupSecurity(db *gorm.DB) error {
	// Enable row level security on sensitive tables
	tables := []string{"users", "patients", "observations"}

	for _, table := range tables {
		// Enable RLS
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", table)).Error; err != nil {
			return fmt.Errorf("failed to enable RLS on table %s: %w", table, err)
		}
	}

	return nil
}

// CloseDB gracefully closes the database connection
func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
