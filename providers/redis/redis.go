package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/adityajoshi12/shedlock-go"
	"github.com/redis/go-redis/v9"
)

// RedisLockProvider implements LockProvider for Redis
type RedisLockProvider struct {
	client *redis.Client
	prefix string
}

// Config holds configuration for Redis lock provider
type Config struct {
	// Client is the Redis client
	Client *redis.Client
	// Prefix is the key prefix for locks (defaults to "shedlock:")
	Prefix string
}

// NewRedisLockProvider creates a new Redis lock provider
func NewRedisLockProvider(config Config) (*RedisLockProvider, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	prefix := config.Prefix
	if prefix == "" {
		prefix = "shedlock:"
	}

	return &RedisLockProvider{
		client: config.Client,
		prefix: prefix,
	}, nil
}

// redisLock represents a lock acquired from Redis
type redisLock struct {
	provider *RedisLockProvider
	name     string
	lockKey  string
	lockID   string
}

// Lock attempts to acquire a lock
func (p *RedisLockProvider) Lock(ctx context.Context, config shedlock.LockConfiguration) (shedlock.SimpleLock, error) {
	lockKey := p.prefix + config.Name
	lockID := generateLockID()

	// Try to acquire the lock using SET with NX (only set if not exists) and PX (expiration in milliseconds)
	expiration := config.LockAtMostFor

	success, err := p.client.SetNX(ctx, lockKey, lockID, expiration).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	// If lock was not acquired, return nil
	if !success {
		return nil, nil
	}

	return &redisLock{
		provider: p,
		name:     config.Name,
		lockKey:  lockKey,
		lockID:   lockID,
	}, nil
}

// Unlock releases the lock
func (l *redisLock) Unlock(ctx context.Context) error {
	// Use Lua script to ensure we only delete the lock if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.provider.client.Eval(ctx, script, []string{l.lockKey}, l.lockID).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result == int64(0) {
		return fmt.Errorf("lock not owned by this process")
	}

	return nil
}

// Extend extends the lock duration
func (l *redisLock) Extend(ctx context.Context, duration time.Duration) error {
	// Use Lua script to extend the lock only if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	durationMs := duration.Milliseconds()
	result, err := l.provider.client.Eval(ctx, script, []string{l.lockKey}, l.lockID, durationMs).Result()
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	if result == int64(0) {
		return fmt.Errorf("lock not owned by this process")
	}

	return nil
}

// generateLockID generates a unique identifier for the lock
func generateLockID() string {
	// In production, you might want to use UUID or a combination of hostname + process ID
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
