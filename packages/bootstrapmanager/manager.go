package bootstrapmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type BootstrappedEvent struct{}

type Events struct {
	Bootstrapped *event.Event[*BootstrappedEvent]
}

// Manager is the bootstrap manager.
type Manager struct {
	events              *Events
	tangle              *tangle.Tangle
	notarizationManager *notarization.Manager
	sync.RWMutex
}

// New creates and returns a new notarization manager.
func New(t *tangle.Tangle, notarizationManager *notarization.Manager) (new *Manager) {
	new = &Manager{
		tangle:              t,
		notarizationManager: notarizationManager,
		events:              &Events{Bootstrapped: event.New[*BootstrappedEvent]()},
	}
	return new
}

func (m *Manager) Setup() {
	m.tangle.TimeManager.Events.Bootstrapped.Attach(event.NewClosure(func(_ *tangle.SyncChangedEvent) {
		m.Lock()
		defer m.Unlock()
		if m.notarizationManager.Bootstrapped() {
			m.events.Bootstrapped.Trigger(&BootstrappedEvent{})
		}
	}))
	m.notarizationManager.Events.Bootstrapped.Attach(event.NewClosure(func(_ *notarization.BootstrappedEvent) {
		m.Lock()
		defer m.Unlock()
		if m.tangle.Bootstrapped() {
			m.events.Bootstrapped.Trigger(&BootstrappedEvent{})
		}
	}))
}

// Bootstrapped returns bool indicating if the node is bootstrapped.
func (m *Manager) Bootstrapped() bool {
	m.RLock()
	defer m.RUnlock()
	return m.tangle.Bootstrapped() && m.notarizationManager.Bootstrapped()
}

// Events returns the events of the manager.
func (m *Manager) Events() *Events {
	return m.events
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
