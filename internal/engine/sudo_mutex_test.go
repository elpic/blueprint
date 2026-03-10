package engine

import (
	"testing"
)

// TestSudoPasswordCacheMutexSafety verifies that passwordCache reads and writes
// from concurrent goroutines do not race. Run with -race to detect data races.
func TestSudoPasswordCacheMutexSafety(t *testing.T) {
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			passwordCache.set("sudo", "password")
		}
		close(done)
	}()
	for i := 0; i < 100; i++ {
		_, _ = passwordCache.get("sudo")
	}
	<-done
}
