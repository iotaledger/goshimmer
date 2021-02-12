package statement

import (
	"context"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/identity"
)

// region Registry /////////////////////////////////////////////////////////////////////////////////////////////////////

// Registry holds the opinions of all the nodes.
type Registry struct {
	nodesView map[identity.ID]*View
	mu        sync.RWMutex
}

// NewRegistry returns a new registry.
func NewRegistry() *Registry {
	return &Registry{
		nodesView: make(map[identity.ID]*View),
	}
}

// NodeView returns the view of the given node, and adds a new view if not present.
func (r *Registry) NodeView(id identity.ID) *View {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.nodesView[id]; !ok {
		r.nodesView[id] = &View{
			NodeID:     id,
			Conflicts:  make(map[transaction.ID]Entry),
			Timestamps: make(map[tangle.MessageID]Entry),
		}
	}

	return r.nodesView[id]
}

// NodesView returns a slice of the views of all nodes.
func (r *Registry) NodesView() []*View {
	r.mu.RLock()
	defer r.mu.RUnlock()

	views := make([]*View, 0, len(r.nodesView))

	for _, v := range r.nodesView {
		views = append(views, v)
	}

	return views
}

// Clean deletes all the entries older than the given duration d.
func (r *Registry) Clean(d time.Duration) {
	now := clock.SyncedTime()

	for _, v := range r.NodesView() {
		v.cMutex.Lock()
		// loop over the conflicts
		for id, c := range v.Conflicts {
			if c.Timestamp.Add(d).Before(now) {
				delete(v.Conflicts, id)
			}
		}
		v.cMutex.Unlock()

		v.tMutex.Lock()
		// loop over the timestamps
		for id, t := range v.Timestamps {
			if t.Timestamp.Add(d).Before(now) {
				delete(v.Timestamps, id)
			}
		}
		v.tMutex.Unlock()
	}
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////

// region Entry /////////////////////////////////////////////////////////////////////////////////////////////////////

// Entry defines the entry of a registry.
type Entry struct {
	Opinions
	Timestamp time.Time
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////

// region View /////////////////////////////////////////////////////////////////////////////////////////////////////

// View holds the node's opinion about conflicts and timestamps.
type View struct {
	NodeID     identity.ID
	Conflicts  map[transaction.ID]Entry
	cMutex     sync.RWMutex
	Timestamps map[tangle.MessageID]Entry
	tMutex     sync.RWMutex
}

// AddConflict appends the given conflict to the given view.
func (v *View) AddConflict(c Conflict) {
	v.cMutex.Lock()
	defer v.cMutex.Unlock()

	if _, ok := v.Conflicts[c.ID]; !ok {
		v.Conflicts[c.ID] = Entry{
			Opinions:  Opinions{c.Opinion},
			Timestamp: clock.SyncedTime(),
		}
		return
	}

	entry := v.Conflicts[c.ID]
	entry.Opinions = append(entry.Opinions, c.Opinion)
	v.Conflicts[c.ID] = entry
}

// AddConflicts appends the given conflicts to the given view.
func (v *View) AddConflicts(conflicts Conflicts) {
	v.cMutex.Lock()
	defer v.cMutex.Unlock()

	for _, c := range conflicts {
		if _, ok := v.Conflicts[c.ID]; !ok {
			v.Conflicts[c.ID] = Entry{
				Opinions:  Opinions{c.Opinion},
				Timestamp: clock.SyncedTime(),
			}
			continue
		}

		entry := v.Conflicts[c.ID]
		entry.Opinions = append(entry.Opinions, c.Opinion)
		v.Conflicts[c.ID] = entry
	}
}

// AddTimestamp appends the given timestamp to the given view.
func (v *View) AddTimestamp(t Timestamp) {
	v.tMutex.Lock()
	defer v.tMutex.Unlock()

	if _, ok := v.Timestamps[t.ID]; !ok {
		v.Timestamps[t.ID] = Entry{
			Opinions:  Opinions{t.Opinion},
			Timestamp: clock.SyncedTime(),
		}
		return
	}

	entry := v.Timestamps[t.ID]
	entry.Opinions = append(entry.Opinions, t.Opinion)
	v.Timestamps[t.ID] = entry
}

// AddTimestamps appends the given timestamps to the given view.
func (v *View) AddTimestamps(timestamps Timestamps) {
	v.tMutex.Lock()
	defer v.tMutex.Unlock()

	for _, t := range timestamps {
		if _, ok := v.Timestamps[t.ID]; !ok {
			v.Timestamps[t.ID] = Entry{
				Opinions:  Opinions{t.Opinion},
				Timestamp: clock.SyncedTime(),
			}
			continue
		}

		entry := v.Timestamps[t.ID]
		entry.Opinions = append(entry.Opinions, t.Opinion)
		v.Timestamps[t.ID] = entry
	}
}

// ConflictOpinion returns the opinion history of a given transaction ID.
func (v *View) ConflictOpinion(id transaction.ID) Opinions {
	v.cMutex.RLock()
	defer v.cMutex.RUnlock()

	if _, ok := v.Conflicts[id]; !ok {
		return Opinions{}
	}

	return v.Conflicts[id].Opinions
}

// TimestampOpinion returns the opinion history of a given message ID.
func (v *View) TimestampOpinion(id tangle.MessageID) Opinions {
	v.tMutex.RLock()
	defer v.tMutex.RUnlock()

	if _, ok := v.Timestamps[id]; !ok {
		return Opinions{}
	}

	return v.Timestamps[id].Opinions
}

// Query retrievs the opinions about the given conflicts and timestamps.
func (v *View) Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (opinion.Opinions, error) {
	answer := opinion.Opinions{}
	for _, id := range conflictIDs {
		ID, err := transaction.IDFromBase58(id)
		if err != nil {
			return answer, err
		}
		o := v.ConflictOpinion(ID)
		opinion := opinion.Unknown
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
		opinion := opinion.Unknown
		if len(o) > 0 {
			opinion = o.Last().Value
		}
		answer = append(answer, opinion)
	}
	return answer, nil
}

// ID returns the nodeID of the given view.
func (v *View) ID() identity.ID {
	return v.NodeID
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////
