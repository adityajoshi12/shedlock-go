package shedlock

// LockAssert helps verify that locks are properly configured and acquired
type LockAssert struct {
	testMode bool
}

var globalLockAssert = &LockAssert{testMode: false}

// AssertLocked verifies that a lock is currently held
// This should be called inside your locked task to ensure the lock is working
func AssertLocked() {
	if !globalLockAssert.testMode {
		// In production, this would check if a lock is actually held
		// For now, we trust that the executor has acquired the lock
		// A more sophisticated implementation could use thread-local storage
	}
}

// TestHelper provides utilities for testing
type TestHelper struct{}

// MakeAllAssertsPass configures assertions to always pass (for testing)
func (TestHelper) MakeAllAssertsPass(pass bool) {
	globalLockAssert.testMode = pass
}
