package logging

import (
	"sync"
	"testing"
	"time"
)

func entry(text string) Entry {
	return Entry{Time: time.Now(), Level: LevelInfo, Message: text}
}

func TestRingBufferStoresInOrder(t *testing.T) {
	ring := NewRingBuffer(4)
	for _, text := range []string{"a", "b", "c"} {
		ring.Append(entry(text))
	}
	snapshot := ring.Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(snapshot))
	}
	for index, want := range []string{"a", "b", "c"} {
		if snapshot[index].Message != want {
			t.Fatalf("entry %d = %q, want %q", index, snapshot[index].Message, want)
		}
	}
}

func TestRingBufferEvictsOldest(t *testing.T) {
	ring := NewRingBuffer(3)
	for _, text := range []string{"a", "b", "c", "d", "e"} {
		ring.Append(entry(text))
	}
	snapshot := ring.Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(snapshot))
	}
	for index, want := range []string{"c", "d", "e"} {
		if snapshot[index].Message != want {
			t.Fatalf("entry %d = %q, want %q", index, snapshot[index].Message, want)
		}
	}
}

func TestRingBufferClear(t *testing.T) {
	ring := NewRingBuffer(4)
	ring.Append(entry("a"))
	ring.Clear()
	if snapshot := ring.Snapshot(); len(snapshot) != 0 {
		t.Fatalf("expected empty snapshot, got %d entries", len(snapshot))
	}
}

func TestRingBufferConcurrentAppendAndSnapshot(t *testing.T) {
	ring := NewRingBuffer(64)
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for index := 0; index < 100; index++ {
				ring.Append(entry("msg"))
				_ = ring.Snapshot()
			}
		}(worker)
	}
	wait.Wait()
	if len(ring.Snapshot()) != 64 {
		t.Fatalf("expected full ring of 64, got %d", len(ring.Snapshot()))
	}
}
