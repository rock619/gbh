package audio

import "sync"

type RingBuffer[T any] struct {
	buffer     []T
	readIndex  int
	writeIndex int
	count      int
	mu         sync.Mutex
}

func NewRingBuffer[T any](size int) *RingBuffer[T] {
	return &RingBuffer[T]{
		buffer: make([]T, size),
	}
}

func (rb *RingBuffer[T]) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

func (rb *RingBuffer[T]) Push(value T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.writeIndex%len(rb.buffer)] = value
	rb.writeIndex++

	if rb.count < len(rb.buffer) {
		rb.count++
		return
	}
	// The oldest sample was overwritten when full, so advance readIndex as well.
	rb.readIndex++
}

func (rb *RingBuffer[T]) Pop() (T, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		var zero T
		return zero, false
	}
	value := rb.buffer[rb.readIndex%len(rb.buffer)]
	rb.readIndex++
	rb.count--
	return value, true
}

func (rb *RingBuffer[T]) DiscardOldest(n int) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if n > rb.count {
		n = rb.count
	}
	rb.readIndex += n
	rb.count -= n
}
