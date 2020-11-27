package statement

import (
	"context"
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
)

// Registry holds the opinions of all the nodes.
type Registry struct {
	nodesView map[string]*View
	mu        sync.RWMutex
}

// View holds the node's opinion about conflicts and timestamps.
type View struct {
	NodeID     string
	Conflicts  map[transaction.ID]Opinions
	cMutex     sync.RWMutex
	Timestamps map[tangle.MessageID]Opinions
	tMutex     sync.RWMutex
}

// NewRegistry returns a new registry.
func NewRegistry() *Registry {
	return &Registry{
		nodesView: make(map[string]*View),
	}
}

// NodeView returns the view of the given node, and adds a new view if not present.
func (r *Registry) NodeView(id string) *View {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.nodesView[id]; !ok {
		r.nodesView[id] = &View{
			NodeID:     id,
			Conflicts:  make(map[transaction.ID]Opinions),
			Timestamps: make(map[tangle.MessageID]Opinions),
		}
	}

	return r.nodesView[id]
}

// NodesView returns a slice of the views of all nodes.
func (r *Registry) NodesView() []*View {
	views := make([]*View, 0)

	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.nodesView {
		views = append(views, v)
	}

	return views
}

// AddConflict appends the given conflict to the given view.
func (v *View) AddConflict(c Conflict) {
	v.cMutex.Lock()
	defer v.cMutex.Unlock()

	if _, ok := v.Conflicts[c.ID]; !ok {
		v.Conflicts[c.ID] = Opinions{c.Opinion}
		return
	}

	v.Conflicts[c.ID] = append(v.Conflicts[c.ID], c.Opinion)
}

// AddTimestamp appends the given timestamp to the given view.
func (v *View) AddTimestamp(t Timestamp) {
	v.tMutex.Lock()
	defer v.tMutex.Unlock()

	if _, ok := v.Timestamps[t.ID]; !ok {
		v.Timestamps[t.ID] = Opinions{t.Opinion}
		return
	}

	v.Timestamps[t.ID] = append(v.Timestamps[t.ID], t.Opinion)
}

// ConflictOpinion returns the opinion history of a given transaction ID.
func (v *View) ConflictOpinion(id transaction.ID) Opinions {
	v.cMutex.RLock()
	defer v.cMutex.RUnlock()

	if _, ok := v.Conflicts[id]; !ok {
		return Opinions{}
	}

	return v.Conflicts[id]
}

// TimestampOpinion returns the opinion history of a given message ID.
func (v *View) TimestampOpinion(id tangle.MessageID) Opinions {
	v.tMutex.RLock()
	defer v.tMutex.RUnlock()

	if _, ok := v.Timestamps[id]; !ok {
		return Opinions{}
	}

	return v.Timestamps[id]
}

// Query retrievs the opinions about the given conflicts and timestamps.
func (v *View) Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (vote.Opinions, error) {
	answer := vote.Opinions{}
	for _, id := range conflictIDs {
		ID, err := transaction.IDFromBase58(id)
		if err != nil {
			return answer, err
		}
		o := v.ConflictOpinion(ID)
		opinion := vote.Unknown
		if len(o) > 0 {
			opinion = o.Last().Value
		}
		answer = append(answer, opinion)
	}
	for _, id := range timestampIDs {
		ID, err := tangle.NewMessageID(id)
		if err != nil {
			return answer, err
		}
		o := v.TimestampOpinion(ID)
		opinion := vote.Unknown
		if len(o) > 0 {
			opinion = o.Last().Value
		}
		answer = append(answer, opinion)
	}
	return answer, nil
}

// ID returns the nodeID of the given view.
func (v *View) ID() string {
	return v.NodeID
}
