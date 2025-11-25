// Package shedlock provides distributed locking for scheduled tasks.
//
// ShedLock ensures that your scheduled tasks are executed at most once at the same time.
// If a task is being executed on one node, it acquires a lock which prevents execution
// of the same task from another node (or thread). If one task is already being executed
// on one node, execution on other nodes does not wait, it is simply skipped.
//
// # Basic Usage
//
// First, create a lock provider (PostgreSQL or Redis):
//
//	provider, err := redis.NewRedisLockProvider(redis.Config{
//	    Client: redisClient,
//	})
//
// Then create a task executor:
//
//	executor := shedlock.NewDefaultLockingTaskExecutor(provider)
//
// Execute your task with a lock:
//
//	err := executor.ExecuteWithLock(ctx, myTask, shedlock.LockConfiguration{
//	    Name:           "myScheduledTask",
//	    LockAtMostFor:  10 * time.Minute,
//	    LockAtLeastFor: 0,
//	})
//
// # Lock Providers
//
// ShedLock supports multiple storage backends:
//   - PostgreSQL (github.com/adityajoshi12/shedlock-go/providers/postgres)
//   - Redis (github.com/adityajoshi12/shedlock-go/providers/redis)
//
// # Lock Configuration
//
// LockAtMostFor specifies how long the lock should be kept in case the executing
// node dies. This is a safety mechanism to prevent deadlocks.
//
// LockAtLeastFor specifies the minimum amount of time for which the lock should
// be kept. This prevents the task from executing too frequently.
package shedlock

import (
	"context"
	"time"
)

// LockConfiguration holds the configuration for acquiring a lock
type LockConfiguration struct {
	// Name is the unique name of the lock
	Name string
	// LockAtMostFor specifies how long the lock should be kept in case the executing node dies
	LockAtMostFor time.Duration
	// LockAtLeastFor specifies minimum amount of time for which the lock should be kept
	LockAtLeastFor time.Duration
}

// SimpleLock represents an acquired lock
type SimpleLock interface {
	// Unlock releases the lock
	Unlock(ctx context.Context) error
	// Extend extends the lock duration (if supported by provider)
	Extend(ctx context.Context, duration time.Duration) error
}

// LockProvider is the interface that must be implemented by lock storage providers
type LockProvider interface {
	// Lock attempts to acquire a lock with the given configuration
	// Returns the lock if successful, nil if lock is already held by another process
	Lock(ctx context.Context, config LockConfiguration) (SimpleLock, error)
}

// LockingTaskExecutor executes tasks with distributed locking
type LockingTaskExecutor interface {
	// ExecuteWithLock executes the given task with a distributed lock
	// If the lock cannot be acquired, the task is skipped
	ExecuteWithLock(ctx context.Context, task func(context.Context) error, config LockConfiguration) error
}

// TaskResult represents the result of task execution
type TaskResult struct {
	// Executed indicates whether the task was executed
	Executed bool
	// LockAcquired indicates whether the lock was acquired
	LockAcquired bool
	// Error contains any error that occurred during execution
	Error error
}
