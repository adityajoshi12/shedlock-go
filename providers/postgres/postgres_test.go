package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/adityajoshi12/shedlock-go"
	_ "github.com/lib/pq"
)

func TestNewPostgresLockProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "nil database",
			config: Config{
				DB: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewPostgresLockProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPostgresLockProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("expected provider to be non-nil")
			}
		})
	}
}

func TestPostgresLockProvider_Lock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Connect to PostgreSQL (requires running PostgreSQL instance)
	db, err := sql.Open("postgres", "postgres://test:test@localhost:5432/shedlock_test?sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	// Check if database is available
	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	provider, err := NewPostgresLockProvider(Config{
		DB:        db,
		TableName: "shedlock_test",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()

	// Create table
	if err := provider.CreateTable(ctx); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Clean up
	defer func() {
		db.ExecContext(ctx, "DROP TABLE IF EXISTS shedlock_test")
	}()

	t.Run("acquire lock successfully", func(t *testing.T) {
		// Clean up any existing locks
		db.ExecContext(ctx, "DELETE FROM shedlock_test WHERE name = $1", "test-lock")

		config := shedlock.LockConfiguration{
			Name:           "test-lock",
			LockAtMostFor:  10 * time.Second,
			LockAtLeastFor: 0,
		}

		lock, err := provider.Lock(ctx, config)
		if err != nil {
			t.Fatalf("failed to acquire lock: %v", err)
		}
		if lock == nil {
			t.Fatal("expected lock to be non-nil")
		}

		// Verify lock exists in database
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM shedlock_test WHERE name = $1", "test-lock").Scan(&count)
		if err != nil {
			t.Fatalf("failed to check lock existence: %v", err)
		}
		if count != 1 {
			t.Error("expected lock to exist in database")
		}

		// Clean up
		if err := lock.Unlock(ctx); err != nil {
			t.Errorf("failed to unlock: %v", err)
		}
	})

	t.Run("cannot acquire lock when already held", func(t *testing.T) {
		db.ExecContext(ctx, "DELETE FROM shedlock_test WHERE name = $1", "test-lock-2")

		config := shedlock.LockConfiguration{
			Name:           "test-lock-2",
			LockAtMostFor:  10 * time.Second,
			LockAtLeastFor: 0,
		}

		// Acquire first lock
		lock1, err := provider.Lock(ctx, config)
		if err != nil {
			t.Fatalf("failed to acquire first lock: %v", err)
		}
		if lock1 == nil {
			t.Fatal("expected first lock to be non-nil")
		}
		defer lock1.Unlock(ctx)

		// Try to acquire second lock
		lock2, err := provider.Lock(ctx, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lock2 != nil {
			t.Error("expected second lock to be nil (already held)")
			lock2.Unlock(ctx)
		}
	})

	t.Run("can acquire lock after it expires", func(t *testing.T) {
		db.ExecContext(ctx, "DELETE FROM shedlock_test WHERE name = $1", "test-lock-3")

		config := shedlock.LockConfiguration{
			Name:           "test-lock-3",
			LockAtMostFor:  100 * time.Millisecond,
			LockAtLeastFor: 0,
		}

		// Acquire first lock
		lock1, err := provider.Lock(ctx, config)
		if err != nil {
			t.Fatalf("failed to acquire first lock: %v", err)
		}
		if lock1 == nil {
			t.Fatal("expected first lock to be non-nil")
		}

		// Wait for lock to expire
		time.Sleep(150 * time.Millisecond)

		// Try to acquire second lock (should succeed after expiration)
		lock2, err := provider.Lock(ctx, config)
		if err != nil {
			t.Fatalf("failed to acquire second lock: %v", err)
		}
		if lock2 == nil {
			t.Error("expected to acquire lock after expiration")
		} else {
			lock2.Unlock(ctx)
		}
	})
}

func TestPostgresLock_Unlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, err := sql.Open("postgres", "postgres://test:test@localhost:5432/shedlock_test?sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	provider, err := NewPostgresLockProvider(Config{
		DB:        db,
		TableName: "shedlock_test",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	provider.CreateTable(ctx)
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS shedlock_test")

	t.Run("unlock removes lock", func(t *testing.T) {
		db.ExecContext(ctx, "DELETE FROM shedlock_test WHERE name = $1", "test-unlock")

		config := shedlock.LockConfiguration{
			Name:           "test-unlock",
			LockAtMostFor:  10 * time.Second,
			LockAtLeastFor: 0,
		}

		lock, err := provider.Lock(ctx, config)
		if err != nil {
			t.Fatalf("failed to acquire lock: %v", err)
		}
		if lock == nil {
			t.Fatal("expected lock to be non-nil")
		}

		// Unlock
		if err := lock.Unlock(ctx); err != nil {
			t.Fatalf("failed to unlock: %v", err)
		}

		// Verify lock is removed
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM shedlock_test WHERE name = $1", "test-unlock").Scan(&count)
		if err != nil {
			t.Fatalf("failed to check lock existence: %v", err)
		}
		if count != 0 {
			t.Error("expected lock to be removed from database")
		}
	})
}

func TestPostgresLock_Extend(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, err := sql.Open("postgres", "postgres://test:test@localhost:5432/shedlock_test?sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	provider, err := NewPostgresLockProvider(Config{
		DB:        db,
		TableName: "shedlock_test",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	provider.CreateTable(ctx)
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS shedlock_test")

	t.Run("extend lock successfully", func(t *testing.T) {
		db.ExecContext(ctx, "DELETE FROM shedlock_test WHERE name = $1", "test-extend")

		config := shedlock.LockConfiguration{
			Name:           "test-extend",
			LockAtMostFor:  1 * time.Second,
			LockAtLeastFor: 0,
		}

		lock, err := provider.Lock(ctx, config)
		if err != nil {
			t.Fatalf("failed to acquire lock: %v", err)
		}
		if lock == nil {
			t.Fatal("expected lock to be non-nil")
		}
		defer lock.Unlock(ctx)

		// Get initial lock_until
		var lockUntil1 time.Time
		err = db.QueryRowContext(ctx, "SELECT lock_until FROM shedlock_test WHERE name = $1", "test-extend").Scan(&lockUntil1)
		if err != nil {
			t.Fatalf("failed to get lock_until: %v", err)
		}

		// Extend lock
		if err := lock.Extend(ctx, 10*time.Second); err != nil {
			t.Fatalf("failed to extend lock: %v", err)
		}

		// Get new lock_until
		var lockUntil2 time.Time
		err = db.QueryRowContext(ctx, "SELECT lock_until FROM shedlock_test WHERE name = $1", "test-extend").Scan(&lockUntil2)
		if err != nil {
			t.Fatalf("failed to get lock_until: %v", err)
		}

		// New lock_until should be greater than initial
		if !lockUntil2.After(lockUntil1) {
			t.Errorf("expected lock_until to increase after extension, got %v -> %v", lockUntil1, lockUntil2)
		}
	})
}

func TestPostgresLockProvider_CreateTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, err := sql.Open("postgres", "postgres://test:test@localhost:5432/shedlock_test?sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	provider, err := NewPostgresLockProvider(Config{
		DB:        db,
		TableName: "shedlock_create_test",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS shedlock_create_test")

	t.Run("create table successfully", func(t *testing.T) {
		if err := provider.CreateTable(ctx); err != nil {
			t.Fatalf("failed to create table: %v", err)
		}

		// Verify table exists
		var exists bool
		err = db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_name = $1
			)
		`, "shedlock_create_test").Scan(&exists)
		if err != nil {
			t.Fatalf("failed to check table existence: %v", err)
		}
		if !exists {
			t.Error("expected table to exist")
		}
	})

	t.Run("create table idempotent", func(t *testing.T) {
		// Creating table again should not error
		if err := provider.CreateTable(ctx); err != nil {
			t.Errorf("expected CreateTable to be idempotent, got error: %v", err)
		}
	})
}

func TestPostgresLock_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, err := sql.Open("postgres", "postgres://test:test@localhost:5432/shedlock_test?sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	provider, err := NewPostgresLockProvider(Config{
		DB:        db,
		TableName: "shedlock_test",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	provider.CreateTable(ctx)
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS shedlock_test")

	t.Run("concurrent lock attempts", func(t *testing.T) {
		db.ExecContext(ctx, "DELETE FROM shedlock_test WHERE name = $1", "test-concurrent")

		config := shedlock.LockConfiguration{
			Name:           "test-concurrent",
			LockAtMostFor:  5 * time.Second,
			LockAtLeastFor: 0,
		}

		// Try to acquire lock from multiple goroutines
		acquiredCount := 0
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				lock, err := provider.Lock(ctx, config)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if lock != nil {
					acquiredCount++
					time.Sleep(100 * time.Millisecond)
					lock.Unlock(ctx)
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Only one should have acquired the lock
		if acquiredCount != 1 {
			t.Errorf("expected 1 lock acquisition, got %d", acquiredCount)
		}
	})
}

func TestGetHostname(t *testing.T) {
	hostname := getHostname()
	if hostname == "" {
		t.Error("expected hostname to be non-empty")
	}
}
