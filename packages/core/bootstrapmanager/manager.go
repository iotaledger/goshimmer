package bootstrapmanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type BootstrappedEvent struct{}

type Events struct {
	Bootstrapped *event.Event[*BootstrappedEvent]
}

// Manager is the bootstrap manager.
type Manager struct {
	Events              *Events
	tangle              *tangleold.Tangle
	notarizationManager *notarization.Manager
}

// New creates and returns a new notarization manager.
func New(t *tangleold.Tangle, notarizationManager *notarization.Manager) (new *Manager) {
	new = &Manager{
		tangle:              t,
		notarizationManager: notarizationManager,
		Events:              &Events{Bootstrapped: event.New[*BootstrappedEvent]()},
	}
	return new
}

func (m *Manager) Setup() {
	m.tangle.TimeManager.Events.Bootstrapped.Attach(event.NewClosure(func(_ *tangleold.BootstrappedEvent) {
		if m.notarizationManager.Bootstrapped() {
			m.Events.Bootstrapped.Trigger(&BootstrappedEvent{})
		}
	}))
	m.notarizationManager.Events.Bootstrapped.Attach(event.NewClosure(func(_ *notarization.BootstrappedEvent) {
		if m.tangle.Bootstrapped() {
			m.Events.Bootstrapped.Trigger(&BootstrappedEvent{})
		}
	}))
}

// Bootstrapped returns bool indicating if the node is bootstrapped.
func (m *Manager) Bootstrapped() bool {
	return m.tangle.Bootstrapped() && m.notarizationManager.Bootstrapped()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
