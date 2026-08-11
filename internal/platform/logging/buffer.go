package logging

import "sync"

// DefaultCapacity is how many recent entries the in-memory log ring keeps.
const DefaultCapacity = 2000

// RingBuffer is a fixed-capacity, concurrency-safe ring of recent log entries.
// New entries overwrite the oldest once the buffer is full.
type RingBuffer struct {
	mu    sync.RWMutex
	items []Entry
	start int
	count int
}

// NewRingBuffer returns an empty ring able to hold capacity entries.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &RingBuffer{items: make([]Entry, capacity)}
}

// Append stores an entry, evicting the oldest one when the ring is full.
func (buffer *RingBuffer) Append(entry Entry) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	index := (buffer.start + buffer.count) % len(buffer.items)
	buffer.items[index] = entry
	if buffer.count < len(buffer.items) {
		buffer.count++
	} else {
		buffer.start = (buffer.start + 1) % len(buffer.items)
	}
}

// Snapshot returns a copy of all stored entries in chronological order.
func (buffer *RingBuffer) Snapshot() []Entry {
	buffer.mu.RLock()
	defer buffer.mu.RUnlock()
	result := make([]Entry, 0, buffer.count)
	for i := 0; i < buffer.count; i++ {
		result = append(result, buffer.items[(buffer.start+i)%len(buffer.items)])
	}
	return result
}

// Clear removes every stored entry.
func (buffer *RingBuffer) Clear() {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.start = 0
	buffer.count = 0
}
