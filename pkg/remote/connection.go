package remote

import (
	"context"
	"sync"
	"time"
)

// ConnectionMonitor handles connection health monitoring and heartbeats
type ConnectionMonitor struct {
	interval       time.Duration
	timeout        time.Duration
	lastHeartbeat  time.Time
	mu             sync.RWMutex
	stopCh         chan struct{}
	healthCallback func(bool) // Called with health status changes
	stopped        bool
}

// NewConnectionMonitor creates a new connection monitor
func NewConnectionMonitor(interval, timeout time.Duration, healthCallback func(bool)) *ConnectionMonitor {
	return &ConnectionMonitor{
		interval:       interval,
		timeout:        timeout,
		lastHeartbeat:  time.Now(),
		stopCh:         make(chan struct{}),
		healthCallback: healthCallback,
	}
}

// Start begins monitoring the connection
func (m *ConnectionMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	healthy := true

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.RLock()
			lastSeen := m.lastHeartbeat
			m.mu.RUnlock()

			// Check if connection is stale
			timeSinceLastHeartbeat := time.Since(lastSeen)
			isHealthy := timeSinceLastHeartbeat < m.timeout

			// If health status changed, notify callback
			if isHealthy != healthy {
				healthy = isHealthy
				if m.healthCallback != nil {
					m.healthCallback(healthy)
				}
			}
		}
	}
}

// Stop stops the connection monitor
func (m *ConnectionMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.stopped {
		close(m.stopCh)
		m.stopped = true
	}
}

// RecordHeartbeat records a successful heartbeat
func (m *ConnectionMonitor) RecordHeartbeat() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastHeartbeat = time.Now()
}

// IsHealthy returns whether the connection is currently healthy
func (m *ConnectionMonitor) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Since(m.lastHeartbeat) < m.timeout
}

// OutputBuffer buffers output during brief disconnects
type OutputBuffer struct {
	maxSize int
	buffer  []string
	mu      sync.RWMutex
}

// NewOutputBuffer creates a new output buffer
func NewOutputBuffer(maxSize int) *OutputBuffer {
	return &OutputBuffer{
		maxSize: maxSize,
		buffer:  make([]string, 0, maxSize),
	}
}

// Add adds a message to the buffer
func (b *OutputBuffer) Add(msg string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.buffer) >= b.maxSize {
		return false // Buffer full
	}

	b.buffer = append(b.buffer, msg)
	return true
}

// Drain returns all buffered messages and clears the buffer
func (b *OutputBuffer) Drain() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	messages := make([]string, len(b.buffer))
	copy(messages, b.buffer)
	b.buffer = b.buffer[:0] // Clear buffer
	return messages
}

// Size returns the current buffer size
func (b *OutputBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.buffer)
}

// IsFull returns whether the buffer is full
func (b *OutputBuffer) IsFull() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.buffer) >= b.maxSize
}

// Clear clears all buffered messages
func (b *OutputBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer = b.buffer[:0]
}
