package context

import (
	"context"
	"testing"
	"time"
)

// Define proper types for context keys to avoid collisions
type testKeyType string
type numberKeyType string
type stringKeyType string
type structKeyType string
type sliceKeyType string

const (
	testKey   = testKeyType("testKey")
	numberKey = numberKeyType("number")
	stringKey = stringKeyType("string")
	structKey = structKeyType("struct")
	sliceKey  = sliceKeyType("slice")
)

func TestWithoutCancel(t *testing.T) {
	// Test 1: Basic functionality - value preservation and cancellation isolation
	t.Run("BasicFunctionality", func(t *testing.T) {
		// Create parent context with values using proper key types
		parent := context.WithValue(context.Background(), testKey, "testValue")
		parent = context.WithValue(parent, numberKey, 42)

		// Create detached context
		detached := WithoutCancel(parent)

		// Verify values are preserved
		if val := detached.Value(testKey); val != "testValue" {
			t.Errorf("Expected testValue, got %v", val)
		}
		if val := detached.Value(numberKey); val != 42 {
			t.Errorf("Expected 42, got %v", val)
		}

		// Verify cancellation is ignored
		if detached.Done() != nil {
			t.Error("Detached context should not be cancellable")
		}
		if detached.Err() != nil {
			t.Error("Detached context should not have error")
		}
	})

	// Test 2: Parent context cancellation does not affect detached context
	t.Run("ParentCancellationIsolation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		ctx = context.WithValue(ctx, testKey, "testValue")

		detached := WithoutCancel(ctx)

		// Cancel parent context
		cancel()

		// Verify detached context is not affected
		select {
		case <-detached.Done():
			t.Error("Detached context should not be canceled")
		default:
			// Expected: detached should not be canceled
		}

		if detached.Err() != nil {
			t.Error("Detached context should not have error")
		}

		// Verify values are still accessible
		if val := detached.Value(testKey); val != "testValue" {
			t.Errorf("Expected testValue after parent cancellation, got %v", val)
		}
	})

	// Test 3: Deadline isolation
	t.Run("DeadlineIsolation", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
		defer cancel()

		detached := WithoutCancel(ctx)

		// Verify detached context has no deadline
		if _, ok := detached.Deadline(); ok {
			t.Error("Detached context should not have deadline")
		}
	})

	// Test 4: Nested usage
	t.Run("NestedUsage", func(t *testing.T) {
		level1Key := testKeyType("level1")
		parent := context.WithValue(context.Background(), level1Key, "value1")

		detached1 := WithoutCancel(parent)
		detached2 := WithoutCancel(detached1)

		// Verify nested context still preserves values
		if val := detached2.Value(level1Key); val != "value1" {
			t.Errorf("Expected value1 in nested context, got %v", val)
		}

		// Verify nested context is also not cancellable
		if detached2.Done() != nil {
			t.Error("Nested detached context should not be cancellable")
		}
	})

	// Test 5: Nil parent handling
	t.Run("NilParentHandling", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for nil parent")
			}
		}()
		// Use a variable to avoid staticcheck SA1012 warning about nil context
		var nilCtx context.Context
		WithoutCancel(nilCtx) // Should panic
	})

	// Test 6: Complex context values
	t.Run("ComplexContextValues", func(t *testing.T) {
		type testStruct struct {
			Name  string
			Value int
		}

		parent := context.WithValue(context.Background(), stringKey, "hello")
		parent = context.WithValue(parent, structKey, testStruct{Name: "test", Value: 123})
		parent = context.WithValue(parent, sliceKey, []int{1, 2, 3})

		detached := WithoutCancel(parent)

		// Verify various types of values are preserved
		if val := detached.Value(stringKey); val != "hello" {
			t.Errorf("Expected hello, got %v", val)
		}

		expectedStruct := testStruct{Name: "test", Value: 123}
		if val := detached.Value(structKey); val != expectedStruct {
			t.Errorf("Expected %+v, got %+v", expectedStruct, val)
		}

		expectedSlice := []int{1, 2, 3}
		if val := detached.Value(sliceKey); !testSliceEqual(val.([]int), expectedSlice) {
			t.Errorf("Expected %+v, got %+v", expectedSlice, val)
		}
	})

	// Test 7: Concurrent access from multiple goroutines
	t.Run("ConcurrentAccess", func(t *testing.T) {
		parent := context.WithValue(context.Background(), testKey, "value")
		detached := WithoutCancel(parent)

		done := make(chan bool, 10)

		// Start multiple goroutines for concurrent access
		for i := 0; i < 10; i++ {
			go func() {
				// Verify value access
				if val := detached.Value(testKey); val != "value" {
					t.Errorf("Concurrent access failed: expected value, got %v", val)
				}
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			select {
			case <-done:
				// Expected success
			case <-time.After(time.Second):
				t.Fatal("Concurrent access test timed out")
			}
		}
	})

	// Test 8: Standard library behavior consistency
	t.Run("StandardLibraryConsistency", func(t *testing.T) {
		// Although we cannot directly test standard library's private types
		// We can verify our implementation conforms to the standard library interface contract

		parent := context.WithValue(context.Background(), testKey, "value")
		detached := WithoutCancel(parent)

		// Verify interface consistency
		var _ context.Context = detached // Compile-time interface verification

		// Verify behavior consistency
		if detached.Done() != nil {
			t.Error("Should return nil Done channel")
		}
		if detached.Err() != nil {
			t.Error("Should return nil error")
		}
		if _, ok := detached.Deadline(); ok {
			t.Error("Should not have deadline")
		}
	})
}

// Helper function: compare if two slices are equal
func testSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}