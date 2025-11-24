package redis

import (
	"context"
	"testing"
	"time"

	"github.com/adityajoshi12/shedlock-go"
	"github.com/redis/go-redis/v9"
)

func TestNewRedisLockProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with client",
			config: Config{
				Client: redis.NewClient(&redis.Options{Addr: "localhost:6379"}),
				Prefix: "test:",
			},
			wantErr: false,
		},
		{
			name: "valid config with default prefix",
			config: Config{
				Client: redis.NewClient(&redis.Options{Addr: "localhost:6379"}),
			},
			wantErr: false,
		},
		{
			name: "nil client",
			config: Config{
				Client: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewRedisLockProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRedisLockProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if provider == nil {
					t.Error("expected provider to be non-nil")
				}
				if provider.prefix == "" {
					t.Error("expected prefix to be set")
				}
				if tt.config.Prefix != "" && provider.prefix != tt.config.Prefix {
					t.Errorf("expected prefix %s, got %s", tt.config.Prefix, provider.prefix)
				}
			}
		})
	}
}

func TestRedisLockProvider_Lock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a Redis client (this requires a running Redis instance)
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	// Check if Redis is available
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Clean up any existing test locks
	testKey := "shedlock:test-lock"
	client.Del(ctx, testKey)

	provider, err := NewRedisLockProvider(Config{
		Client: client,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	t.Run("acquire lock successfully", func(t *testing.T) {
		defer client.Del(ctx, testKey)

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

		// Verify lock exists in Redis
		exists, err := client.Exists(ctx, testKey).Result()
		if err != nil {
			t.Fatalf("failed to check lock existence: %v", err)
		}
		if exists != 1 {
			t.Error("expected lock to exist in Redis")
		}

		// Clean up
		if err := lock.Unlock(ctx); err != nil {
			t.Errorf("failed to unlock: %v", err)
		}
	})

	t.Run("cannot acquire lock when already held", func(t *testing.T) {
		defer client.Del(ctx, testKey)

		config := shedlock.LockConfiguration{
			Name:           "test-lock",
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
		defer client.Del(ctx, testKey)

		config := shedlock.LockConfiguration{
			Name:           "test-lock",
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

func TestRedisLock_Unlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	provider, err := NewRedisLockProvider(Config{
		Client: client,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	testKey := "shedlock:test-unlock"
	defer client.Del(ctx, testKey)

	t.Run("unlock removes lock", func(t *testing.T) {
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
		exists, err := client.Exists(ctx, testKey).Result()
		if err != nil {
			t.Fatalf("failed to check lock existence: %v", err)
		}
		if exists != 0 {
			t.Error("expected lock to be removed from Redis")
		}
	})

	t.Run("unlock with different lock ID fails gracefully", func(t *testing.T) {
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
		defer client.Del(ctx, testKey)

		// Manually change the lock value in Redis
		client.Set(ctx, testKey, "different-lock-id", 10*time.Second)

		// Try to unlock - should fail because we don't own it
		err = lock.Unlock(ctx)
		if err == nil {
			t.Error("expected error when unlocking lock owned by different process")
		}
	})
}

func TestRedisLock_Extend(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	provider, err := NewRedisLockProvider(Config{
		Client: client,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	testKey := "shedlock:test-extend"
	defer client.Del(ctx, testKey)

	t.Run("extend lock successfully", func(t *testing.T) {
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

		// Get initial TTL
		ttl1, err := client.TTL(ctx, testKey).Result()
		if err != nil {
			t.Fatalf("failed to get TTL: %v", err)
		}

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Extend lock
		if err := lock.Extend(ctx, 5*time.Second); err != nil {
			t.Fatalf("failed to extend lock: %v", err)
		}

		// Get new TTL
		ttl2, err := client.TTL(ctx, testKey).Result()
		if err != nil {
			t.Fatalf("failed to get TTL: %v", err)
		}

		// New TTL should be greater than initial TTL
		if ttl2 <= ttl1 {
			t.Errorf("expected TTL to increase after extension, got %v -> %v", ttl1, ttl2)
		}
	})

	t.Run("extend fails when lock not owned", func(t *testing.T) {
		config := shedlock.LockConfiguration{
			Name:           "test-extend",
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
		defer client.Del(ctx, testKey)

		// Manually change the lock value in Redis
		client.Set(ctx, testKey, "different-lock-id", 10*time.Second)

		// Try to extend - should fail because we don't own it
		err = lock.Extend(ctx, 5*time.Second)
		if err == nil {
			t.Error("expected error when extending lock owned by different process")
		}
	})
}

func TestRedisLock_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	provider, err := NewRedisLockProvider(Config{
		Client: client,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	testKey := "shedlock:test-concurrent"
	defer client.Del(ctx, testKey)

	t.Run("concurrent lock attempts", func(t *testing.T) {
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

func TestGenerateLockID(t *testing.T) {
	// Test that generateLockID returns unique IDs
	id1 := generateLockID()
	time.Sleep(1 * time.Millisecond)
	id2 := generateLockID()

	if id1 == id2 {
		t.Error("expected different lock IDs")
	}

	if id1 == "" || id2 == "" {
		t.Error("expected non-empty lock IDs")
	}
}
