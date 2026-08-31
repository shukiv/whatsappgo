package events

import "sync"

type Event struct {
	Name string `json:"event"`
	Data any    `json:"data,omitempty"`
}

type Broker struct {
	mu   sync.RWMutex
	next uint64
	subs map[uint64]chan Event
}

func New() *Broker { return &Broker{subs: make(map[uint64]chan Event)} }

func (b *Broker) Publish(evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (b *Broker) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer < 1 {
		buffer = 32
	}
	b.mu.Lock()
	b.next++
	id := b.next
	ch := make(chan Event, buffer)
	b.subs[id] = ch
	b.mu.Unlock()
	var once sync.Once
	return ch, func() { once.Do(func() { b.mu.Lock(); delete(b.subs, id); close(ch); b.mu.Unlock() }) }
}
