package datachannel

import (
	"container/list"
	"fmt"
	"sync"
)

// ListMessageBuffer holds outgoing messages in a linked list, generic over type T.
type ListMessageBuffer[T any] struct {
	messages *list.List
	capacity int
	mutex    sync.RWMutex
}

// NewListMessageBuffer creates a new ListMessageBuffer with the given capacity for type T.
func NewListMessageBuffer[T any](capacity int) *ListMessageBuffer[T] {
	return &ListMessageBuffer[T]{
		messages: list.New(),
		capacity: capacity,
	}
}

// Front returns the first item of type T from the list and true, or the zero value of T and false if the list is empty.
func (b *ListMessageBuffer[T]) Front() (T, bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	var zero T

	e := b.messages.Front()
	if e == nil {
		return zero, false // List is empty
	}

	item, ok := e.Value.(T)
	if !ok {
		panic(fmt.Sprintf("Type assertion failed: expected %T, got %T", zero, e.Value))
	}

	return item, true
}

// ForcePushBack removes first T from if full and adds given T at the end.
func (b *ListMessageBuffer[T]) ForcePushBack(item T) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.messages.Len() == b.capacity {
		b.messages.Remove(b.messages.Front())
	}

	b.messages.PushBack(item)
}

// RemoveIf iterates through the list and removes the first element for which the predicate returns true.
// It returns the removed item (of type T) and true if an element was removed, otherwise the zero value of T and false.
func (b *ListMessageBuffer[T]) RemoveIf(predicate func(T) bool) (T, bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var zero T

	for e := b.messages.Front(); e != nil; e = e.Next() {
		item, ok := e.Value.(T)
		if !ok {
			panic(fmt.Sprintf("Type assertion failed: expected %T, got %T", zero, e.Value))
		}

		if predicate(item) {
			b.messages.Remove(e)

			return item, true
		}
	}

	return zero, false
}
