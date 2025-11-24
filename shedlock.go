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

