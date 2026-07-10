package commands

import "testing"

func TestOwnershipGuardCleansOnceWhileOwned(t *testing.T) {
	calls := 0
	guard := newOwnershipGuard(func() {
		calls++
	})

	guard.clean()
	guard.clean()

	if calls != 1 {
		t.Errorf("cleanup calls = %d, want 1", calls)
	}
}

func TestOwnershipGuardReleaseDisarmsCleanup(t *testing.T) {
	calls := 0
	guard := newOwnershipGuard(func() {
		calls++
	})

	guard.release()
	guard.clean()

	if calls != 0 {
		t.Errorf("cleanup calls = %d, want 0", calls)
	}
}
