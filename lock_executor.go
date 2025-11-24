package shedlock

import (
	"context"
	"fmt"
	"log"
	"time"
)

// DefaultLockingTaskExecutor is the default implementation of LockingTaskExecutor
type DefaultLockingTaskExecutor struct {
	lockProvider LockProvider
	logger       Logger
}

// Logger is a simple logging interface
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// DefaultLogger is a simple logger implementation
type DefaultLogger struct{}

func (l *DefaultLogger) Debug(msg string, keysAndValues ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, keysAndValues)
}

func (l *DefaultLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Printf("[INFO] %s %v", msg, keysAndValues)
}

func (l *DefaultLogger) Error(msg string, keysAndValues ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, keysAndValues)
}

// NewDefaultLockingTaskExecutor creates a new DefaultLockingTaskExecutor
func NewDefaultLockingTaskExecutor(provider LockProvider) *DefaultLockingTaskExecutor {
	return &DefaultLockingTaskExecutor{
		lockProvider: provider,
		logger:       &DefaultLogger{},
	}
}

// NewDefaultLockingTaskExecutorWithLogger creates a new DefaultLockingTaskExecutor with a custom logger
func NewDefaultLockingTaskExecutorWithLogger(provider LockProvider, logger Logger) *DefaultLockingTaskExecutor {
	return &DefaultLockingTaskExecutor{
		lockProvider: provider,
		logger:       logger,
	}
}

// ExecuteWithLock executes the given task with a distributed lock
func (e *DefaultLockingTaskExecutor) ExecuteWithLock(
	ctx context.Context,
	task func(context.Context) error,
	config LockConfiguration,
) error {
	// Validate configuration
	if config.Name == "" {
		return fmt.Errorf("lock name cannot be empty")
	}
	if config.LockAtMostFor <= 0 {
		return fmt.Errorf("lockAtMostFor must be positive")
	}
	if config.LockAtLeastFor < 0 {
		return fmt.Errorf("lockAtLeastFor cannot be negative")
	}

	e.logger.Debug("Attempting to acquire lock", "name", config.Name)

	// Try to acquire the lock
	lock, err := e.lockProvider.Lock(ctx, config)
	if err != nil {
		e.logger.Error("Error acquiring lock", "name", config.Name, "error", err)
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// If lock is nil, it means another process holds the lock
	if lock == nil {
		e.logger.Debug("Lock is held by another process, skipping execution", "name", config.Name)
		return nil
	}

	e.logger.Info("Lock acquired successfully", "name", config.Name)

	// Ensure lock is released
	defer func() {
		unlockCtx := context.Background() // Use background context for cleanup
		if err := lock.Unlock(unlockCtx); err != nil {
			e.logger.Error("Error releasing lock", "name", config.Name, "error", err)
		} else {
			e.logger.Debug("Lock released successfully", "name", config.Name)
		}
	}()

	// Execute the task
	taskStart := time.Now()
	taskErr := task(ctx)
	taskDuration := time.Since(taskStart)

	if taskErr != nil {
		e.logger.Error("Task execution failed", "name", config.Name, "error", taskErr, "duration", taskDuration)
		return fmt.Errorf("task execution failed: %w", taskErr)
	}

	e.logger.Info("Task executed successfully", "name", config.Name, "duration", taskDuration)

	// If lockAtLeastFor is set, wait for the remaining time
	if config.LockAtLeastFor > 0 && taskDuration < config.LockAtLeastFor {
		remainingTime := config.LockAtLeastFor - taskDuration
		e.logger.Debug("Waiting for lockAtLeastFor period", "name", config.Name, "remainingTime", remainingTime)
		
		select {
		case <-time.After(remainingTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

