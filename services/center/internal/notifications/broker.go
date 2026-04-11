package notifications

import (
	"sync"

	notificationdto "mare/shared/contracts/dto/notification"
)

type Broker struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[int]chan notificationdto.StreamEvent
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[int]chan notificationdto.StreamEvent),
	}
}

func (b *Broker) Subscribe() (<-chan notificationdto.StreamEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	ch := make(chan notificationdto.StreamEvent, 32)
	b.subscribers[id] = ch

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subscriber, ok := b.subscribers[id]
		if !ok {
			return
		}
		delete(b.subscribers, id)
		close(subscriber)
	}

	return ch, cancel
}

func (b *Broker) Publish(event notificationdto.StreamEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}
