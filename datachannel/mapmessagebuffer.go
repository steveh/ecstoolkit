package datachannel

import "sync"

// MapMessageBuffer represents a buffer for messages using a map data structure.
type MapMessageBuffer[K comparable, V any] struct {
	messages map[K]V
	capacity int
	mutex    sync.RWMutex
}

// NewMapMessageBuffer creates a new MapMessageBuffer with the given capacity for type T.
func NewMapMessageBuffer[K comparable, V any](capacity int) *MapMessageBuffer[K, V] {
	return &MapMessageBuffer[K, V]{
		messages: make(map[K]V),
		capacity: capacity,
	}
}

// Get returns the item of type T at the given index and true, or the zero value of T and false if the index is not found.
func (b *MapMessageBuffer[K, V]) Get(idx K) (V, bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	item, ok := b.messages[idx]

	return item, ok
}

// SetUnlessFull sets the item at the given index if the buffer is not full.
func (b *MapMessageBuffer[K, V]) SetUnlessFull(idx K, item V) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.messages) == b.capacity {
		return false // Buffer is full
	}

	b.messages[idx] = item

	return true
}

// Remove removes the item at the given index and returns it along with a boolean indicating if it was found.
func (b *MapMessageBuffer[K, V]) Remove(idx K) (V, bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	item, ok := b.messages[idx]
	if ok {
		delete(b.messages, idx)
	}

	return item, ok
}

// IsFull checks if the buffer is full.
func (b *MapMessageBuffer[K, V]) IsFull() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return len(b.messages) == b.capacity
}
