package jobs

import (
	"sync"

	jobdto "mare/shared/contracts/dto/job"
)

type Broker struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[int]subscriber
}

type subscriber struct {
	events chan jobdto.StreamEvent
	jobID  string
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[int]subscriber),
	}
}

func (b *Broker) Subscribe(jobID string) (<-chan jobdto.StreamEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	events := make(chan jobdto.StreamEvent, 32)
	b.subscribers[id] = subscriber{
		events: events,
		jobID:  jobID,
	}

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		sub, ok := b.subscribers[id]
		if !ok {
			return
		}
		delete(b.subscribers, id)
		close(sub.events)
	}

	return events, cancel
}

func (b *Broker) Publish(event jobdto.StreamEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		if sub.jobID != "" && sub.jobID != event.JobID {
			continue
		}
		select {
		case sub.events <- event:
		default:
		}
	}
}
