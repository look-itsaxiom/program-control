// Package events is the in-process fan-out behind the live board: one publisher per status
// change, one subscriber per open stream. It is deliberately not durable. A client that was not
// connected when a change happened catches up through the audit trail, not through this.
package events

import (
	"sync"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// bufferPerSubscriber is how far behind one slow stream may fall before it starts missing
// updates. Dropping is the right failure here: one client with a stalled connection must never
// hold up the write that produced the change.
const bufferPerSubscriber = 32

type subscription struct {
	programID string
	updates   chan store.Task
}

// Broker fans task updates out to whoever is watching that task's program.
type Broker struct {
	mu     sync.Mutex
	nextID int
	subs   map[int]*subscription
}

// New returns an empty broker.
func New() *Broker {
	return &Broker{subs: map[int]*subscription{}}
}

// Subscribe returns a channel of updates for one program and the function that stops the
// subscription. The caller must call it, normally with defer, or the subscription leaks.
func (b *Broker) Subscribe(programID string) (<-chan store.Task, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	sub := &subscription{programID: programID, updates: make(chan store.Task, bufferPerSubscriber)}
	b.subs[id] = sub

	return sub.updates, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(sub.updates)
		}
	}
}

// Publish hands a changed task to every stream watching its program. It never blocks: a
// subscriber whose buffer is full misses this update rather than delaying the publisher.
func (b *Broker) Publish(task store.Task) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subs {
		if sub.programID != task.ProgramID {
			continue
		}
		select {
		case sub.updates <- task:
		default:
		}
	}
}
