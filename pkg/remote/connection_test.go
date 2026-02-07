package remote

import (
	"context"
	"testing"
	"time"
)

func TestConnectionMonitor(t *testing.T) {
	t.Run("Health detection", func(t *testing.T) {
		healthChanges := make(chan bool, 10)

		monitor := NewConnectionMonitor(
			10*time.Millisecond, // Check every 10ms
			30*time.Millisecond, // Timeout after 30ms
			func(healthy bool) {
				healthChanges <- healthy
			},
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start monitoring
		go monitor.Start(ctx)

		// Initially healthy (just created)
		if !monitor.IsHealthy() {
			t.Errorf("Monitor should be healthy initially")
		}

		// Wait for timeout to trigger unhealthy state
		time.Sleep(50 * time.Millisecond)

		if monitor.IsHealthy() {
			t.Errorf("Monitor should be unhealthy after timeout")
		}

		// Record a heartbeat
		monitor.RecordHeartbeat()

		if !monitor.IsHealthy() {
			t.Errorf("Monitor should be healthy after heartbeat")
		}

		monitor.Stop()
	})

	t.Run("Stop prevents further monitoring", func(t *testing.T) {
		monitor := NewConnectionMonitor(10*time.Millisecond, 30*time.Millisecond, nil)

		ctx := context.Background()
		go monitor.Start(ctx)

		time.Sleep(5 * time.Millisecond)
		monitor.Stop()

		// Recording heartbeat after stop shouldn't cause issues
		monitor.RecordHeartbeat()

		// Multiple stops shouldn't panic
		monitor.Stop()
		monitor.Stop()
	})
}

func TestOutputBuffer(t *testing.T) {
	t.Run("Add and drain", func(t *testing.T) {
		buffer := NewOutputBuffer(5)

		// Add messages
		buffer.Add("message 1")
		buffer.Add("message 2")
		buffer.Add("message 3")

		if buffer.Size() != 3 {
			t.Errorf("Expected size 3, got %d", buffer.Size())
		}

		if buffer.IsFull() {
			t.Errorf("Buffer should not be full")
		}

		// Drain messages
		messages := buffer.Drain()
		if len(messages) != 3 {
			t.Errorf("Expected 3 messages, got %d", len(messages))
		}

		if buffer.Size() != 0 {
			t.Errorf("Buffer should be empty after drain")
		}
	})

	t.Run("Buffer full", func(t *testing.T) {
		buffer := NewOutputBuffer(2)

		success := buffer.Add("message 1")
		if !success {
			t.Errorf("First add should succeed")
		}

		success = buffer.Add("message 2")
		if !success {
			t.Errorf("Second add should succeed")
		}

		if !buffer.IsFull() {
			t.Errorf("Buffer should be full")
		}

		// Try to add when full
		success = buffer.Add("message 3")
		if success {
			t.Errorf("Add should fail when buffer is full")
		}

		if buffer.Size() != 2 {
			t.Errorf("Size should remain 2")
		}
	})

	t.Run("Clear buffer", func(t *testing.T) {
		buffer := NewOutputBuffer(5)

		buffer.Add("message 1")
		buffer.Add("message 2")
		buffer.Add("message 3")

		buffer.Clear()

		if buffer.Size() != 0 {
			t.Errorf("Buffer should be empty after clear")
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		buffer := NewOutputBuffer(100)

		done := make(chan bool)

		// Multiple goroutines adding to buffer
		for i := 0; i < 10; i++ {
			go func(id int) {
				for j := 0; j < 10; j++ {
					buffer.Add("message")
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should have 100 messages (all successful)
		if buffer.Size() != 100 {
			t.Errorf("Expected 100 messages, got %d", buffer.Size())
		}
	})
}
