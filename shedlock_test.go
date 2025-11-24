package shedlock

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockLockProvider is a mock implementation of LockProvider for testing
type MockLockProvider struct {
	lockAcquired bool
	shouldFail   bool
}

func (m *MockLockProvider) Lock(ctx context.Context, config LockConfiguration) (SimpleLock, error) {
	if m.shouldFail {
		return nil, errors.New("mock error")
	}
	if m.lockAcquired {
		return nil, nil // Lock already held
	}
	m.lockAcquired = true
	return &MockLock{provider: m}, nil
}

type MockLock struct {
	provider *MockLockProvider
	unlocked bool
}

func (m *MockLock) Unlock(ctx context.Context) error {
	m.unlocked = true
	m.provider.lockAcquired = false
	return nil
}

func (m *MockLock) Extend(ctx context.Context, duration time.Duration) error {
	return nil
}

func TestLockConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  LockConfiguration
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: LockConfiguration{
				Name:           "test-lock",
				LockAtMostFor:  10 * time.Second,
				LockAtLeastFor: 0,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			config: LockConfiguration{
				Name:           "",
				LockAtMostFor:  10 * time.Second,
				LockAtLeastFor: 0,
			},
			wantErr: true,
		},
		{
			name: "zero LockAtMostFor",
			config: LockConfiguration{
				Name:           "test-lock",
				LockAtMostFor:  0,
				LockAtLeastFor: 0,
			},
			wantErr: true,
		},
		{
			name: "negative LockAtLeastFor",
			config: LockConfiguration{
				Name:           "test-lock",
				LockAtMostFor:  10 * time.Second,
				LockAtLeastFor: -1 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &MockLockProvider{}
			executor := NewDefaultLockingTaskExecutor(provider)

			task := func(ctx context.Context) error {
				return nil
			}

			err := executor.ExecuteWithLock(context.Background(), task, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteWithLock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecuteWithLockSuccess(t *testing.T) {
	provider := &MockLockProvider{}
	executor := NewDefaultLockingTaskExecutor(provider)

	taskExecuted := false
	task := func(ctx context.Context) error {
		taskExecuted = true
		return nil
	}

	config := LockConfiguration{
		Name:           "test-lock",
		LockAtMostFor:  10 * time.Second,
		LockAtLeastFor: 0,
	}

	err := executor.ExecuteWithLock(context.Background(), task, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !taskExecuted {
		t.Error("task was not executed")
	}

	if provider.lockAcquired {
		t.Error("lock was not released")
	}
}

func TestExecuteWithLockAlreadyHeld(t *testing.T) {
	provider := &MockLockProvider{lockAcquired: true}
	executor := NewDefaultLockingTaskExecutor(provider)

	taskExecuted := false
	task := func(ctx context.Context) error {
		taskExecuted = true
		return nil
	}

	config := LockConfiguration{
		Name:           "test-lock",
		LockAtMostFor:  10 * time.Second,
		LockAtLeastFor: 0,
	}

	err := executor.ExecuteWithLock(context.Background(), task, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if taskExecuted {
		t.Error("task should not have been executed when lock is held")
	}
}

func TestExecuteWithLockError(t *testing.T) {
	provider := &MockLockProvider{shouldFail: true}
	executor := NewDefaultLockingTaskExecutor(provider)

	task := func(ctx context.Context) error {
		return nil
	}

	config := LockConfiguration{
		Name:           "test-lock",
		LockAtMostFor:  10 * time.Second,
		LockAtLeastFor: 0,
	}

	err := executor.ExecuteWithLock(context.Background(), task, config)
	if err == nil {
		t.Error("expected error but got none")
	}
}

func TestExecuteWithLockTaskError(t *testing.T) {
	provider := &MockLockProvider{}
	executor := NewDefaultLockingTaskExecutor(provider)

	expectedErr := errors.New("task error")
	task := func(ctx context.Context) error {
		return expectedErr
	}

	config := LockConfiguration{
		Name:           "test-lock",
		LockAtMostFor:  10 * time.Second,
		LockAtLeastFor: 0,
	}

	err := executor.ExecuteWithLock(context.Background(), task, config)
	if err == nil {
		t.Error("expected error but got none")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if provider.lockAcquired {
		t.Error("lock was not released after task error")
	}
}

func TestExecuteWithLockAtLeastFor(t *testing.T) {
	provider := &MockLockProvider{}
	executor := NewDefaultLockingTaskExecutor(provider)

	task := func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	config := LockConfiguration{
		Name:           "test-lock",
		LockAtMostFor:  10 * time.Second,
		LockAtLeastFor: 100 * time.Millisecond,
	}

	start := time.Now()
	err := executor.ExecuteWithLock(context.Background(), task, config)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if duration < 100*time.Millisecond {
		t.Errorf("expected duration >= 100ms, got %v", duration)
	}
}

func TestLockAssert(t *testing.T) {
	// Test that LockAssert doesn't panic
	AssertLocked()

	// Test TestHelper
	TestHelper{}.MakeAllAssertsPass(true)
	AssertLocked()

	TestHelper{}.MakeAllAssertsPass(false)
	AssertLocked()
}

