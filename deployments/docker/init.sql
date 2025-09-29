-- Initialize database with proper settings
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create indexes for better performance
-- These will be created by the application, but we can pre-create some
-- CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_patients_name_gin ON patients USING GIN (name);

-- Set timezone
SET timezone = 'UTC';

-- Configure connection settings
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET log_min_duration_statement = 1000;

-- Reload configuration
SELECT pg_reload_conf();