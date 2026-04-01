package models

import (
	"testing"
	"time"
)

// TestHistory tests the History type and its basic operations.
func TestHistory(t *testing.T) {
	start := time.Now()

	// Test creating a new History
	h := make(History)

	// Test adding entries
	h["test1"] = true
	h["test2"] = false

	// Test reading entries
	if h["test1"] != true {
		t.Errorf("Expected h[\"test1\"] to be true, got %v", h["test1"])
	}

	if h["test2"] != false {
		t.Errorf("Expected h[\"test2\"] to be false, got %v", h["test2"])
	}

	// Test non-existent key (should return zero value)
	if h["nonexistent"] != false {
		t.Errorf("Expected h[\"nonexistent\"] to be false, got %v", h["nonexistent"])
	}

	duration := time.Since(start)

	// Should be extremely fast (< 10μs)
	if duration > 10*time.Millisecond {
		t.Errorf("Test took %v, expected < 10μs for simple map operations", duration)
	}
}

// TestHistoryOperations tests various map operations on History.
func TestHistoryOperations(t *testing.T) {
	tests := []struct {
		name      string
		operation func(History) (interface{}, bool)
		expected  interface{}
		exists    bool
	}{
		{
			name: "add and retrieve true",
			operation: func(h History) (interface{}, bool) {
				h["key1"] = true
				val, exists := h["key1"]
				return val, exists
			},
			expected: true,
			exists:   true,
		},
		{
			name: "add and retrieve false",
			operation: func(h History) (interface{}, bool) {
				h["key2"] = false
				val, exists := h["key2"]
				return val, exists
			},
			expected: false,
			exists:   true,
		},
		{
			name: "check non-existent key",
			operation: func(h History) (interface{}, bool) {
				val, exists := h["nonexistent"]
				return val, exists
			},
			expected: false,
			exists:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			h := make(History)
			result, exists := tt.operation(h)

			duration := time.Since(start)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}

			if exists != tt.exists {
				t.Errorf("Expected exists=%v, got %v", tt.exists, exists)
			}

			// Should be extremely fast (< 10μs)
			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10μs for simple map operations", duration)
			}
		})
	}
}

// TestHistoryLength tests the length operations on History.
func TestHistoryLength(t *testing.T) {
	start := time.Now()

	h := make(History)

	// Test empty history
	if len(h) != 0 {
		t.Errorf("Expected empty history to have length 0, got %d", len(h))
	}

	// Add some entries
	h["entry1"] = true
	h["entry2"] = false
	h["entry3"] = true

	// Test length after additions
	if len(h) != 3 {
		t.Errorf("Expected history to have length 3, got %d", len(h))
	}

	// Remove an entry
	delete(h, "entry2")

	// Test length after deletion
	if len(h) != 2 {
		t.Errorf("Expected history to have length 2 after deletion, got %d", len(h))
	}

	duration := time.Since(start)

	// Should be extremely fast (< 10μs)
	if duration > 10*time.Millisecond {
		t.Errorf("Test took %v, expected < 10μs for simple map operations", duration)
	}
}

// TestHistoryIteration tests iterating over History.
func TestHistoryIteration(t *testing.T) {
	start := time.Now()

	h := History{
		"key1": true,
		"key2": false,
		"key3": true,
	}

	count := 0
	trueCount := 0
	falseCount := 0

	for key, value := range h {
		count++
		if value {
			trueCount++
		} else {
			falseCount++
		}

		// Verify key is one of the expected keys
		if key != "key1" && key != "key2" && key != "key3" {
			t.Errorf("Unexpected key in iteration: %s", key)
		}
	}

	if count != 3 {
		t.Errorf("Expected to iterate over 3 items, got %d", count)
	}

	if trueCount != 2 {
		t.Errorf("Expected 2 true values, got %d", trueCount)
	}

	if falseCount != 1 {
		t.Errorf("Expected 1 false value, got %d", falseCount)
	}

	duration := time.Since(start)

	// Should be extremely fast (< 50μs)
	if duration > 10*time.Millisecond {
		t.Errorf("Test took %v, expected < 50μs for simple iteration", duration)
	}
}

// BenchmarkHistoryOperations benchmarks basic History operations.
func BenchmarkHistoryOperations(b *testing.B) {
	h := make(History)

	for i := 0; i < b.N; i++ {
		h["test"] = true
		_ = h["test"]
	}
}

// BenchmarkHistoryIteration benchmarks History iteration.
func BenchmarkHistoryIteration(b *testing.B) {
	h := History{
		"key1": true,
		"key2": false,
		"key3": true,
		"key4": false,
		"key5": true,
	}

	for i := 0; i < b.N; i++ {
		for range h {
			// Just iterate
		}
	}
}
