package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adityajoshi12/shedlock-go"
	_ "github.com/lib/pq" // PostgreSQL driver
)

const (
	// DefaultTableName is the default name for the shedlock table
	DefaultTableName = "shedlock"
)

// PostgresLockProvider implements LockProvider for PostgreSQL
type PostgresLockProvider struct {
	db        *sql.DB
	tableName string
}

// Config holds configuration for PostgreSQL lock provider
type Config struct {
	// DB is the database connection
	DB *sql.DB
	// TableName is the name of the lock table (defaults to "shedlock")
	TableName string
}

// NewPostgresLockProvider creates a new PostgreSQL lock provider
func NewPostgresLockProvider(config Config) (*PostgresLockProvider, error) {
	if config.DB == nil {
		return nil, fmt.Errorf("database connection cannot be nil")
	}

	tableName := config.TableName
	if tableName == "" {
		tableName = DefaultTableName
	}

	provider := &PostgresLockProvider{
		db:        config.DB,
		tableName: tableName,
	}

	return provider, nil
}

// CreateTable creates the shedlock table if it doesn't exist
func (p *PostgresLockProvider) CreateTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			name VARCHAR(64) PRIMARY KEY,
			lock_until TIMESTAMP NOT NULL,
			locked_at TIMESTAMP NOT NULL,
			locked_by VARCHAR(255) NOT NULL
		)
	`, p.tableName)

	_, err := p.db.ExecContext(ctx, query)
	return err
}

// postgresLock represents a lock acquired from PostgreSQL
type postgresLock struct {
	provider *PostgresLockProvider
	name     string
	lockedBy string
}

// Lock attempts to acquire a lock
func (p *PostgresLockProvider) Lock(ctx context.Context, config shedlock.LockConfiguration) (shedlock.SimpleLock, error) {
	now := time.Now()
	lockUntil := now.Add(config.LockAtMostFor)
	lockedBy := getHostname()

	// Try to insert a new lock or update if expired
	query := fmt.Sprintf(`
		INSERT INTO %s (name, lock_until, locked_at, locked_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE SET
			lock_until = EXCLUDED.lock_until,
			locked_at = EXCLUDED.locked_at,
			locked_by = EXCLUDED.locked_by
		WHERE %s.lock_until <= $3
	`, p.tableName, p.tableName)

	result, err := p.db.ExecContext(ctx, query, config.Name, lockUntil, now, lockedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows were affected, the lock is held by another process
	if rowsAffected == 0 {
		return nil, nil
	}

	return &postgresLock{
		provider: p,
		name:     config.Name,
		lockedBy: lockedBy,
	}, nil
}

// Unlock releases the lock
func (l *postgresLock) Unlock(ctx context.Context) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE name = $1 AND locked_by = $2
	`, l.provider.tableName)

	_, err := l.provider.db.ExecContext(ctx, query, l.name, l.lockedBy)
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	return nil
}

// Extend extends the lock duration
func (l *postgresLock) Extend(ctx context.Context, duration time.Duration) error {
	newLockUntil := time.Now().Add(duration)

	query := fmt.Sprintf(`
		UPDATE %s
		SET lock_until = $1
		WHERE name = $2 AND locked_by = $3
	`, l.provider.tableName)

	result, err := l.provider.db.ExecContext(ctx, query, newLockUntil, l.name, l.lockedBy)
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("lock not found or not owned by this process")
	}

	return nil
}

// getHostname returns a hostname identifier for this process
func getHostname() string {
	hostname := "unknown"
	// This is a simplified version, in production you might want to use os.Hostname()
	// and append process ID
	return hostname
}
