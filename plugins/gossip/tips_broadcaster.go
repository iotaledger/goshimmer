package gossip

import (
	"container/list"
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"
)

const (
	// the name of the tips broadcaster worker
	tipsBroadcasterName = PluginName + "[TipsBroadcaster]"
)

var tips = tiplist{dict: make(map[message.Id]*list.Element)}

type tiplist struct {
	mu sync.Mutex

	dict     map[message.Id]*list.Element
	list     list.List
	iterator *list.Element
}

func (s *tiplist) AddTip(id message.Id) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, contains := s.dict[id]; contains {
		return
	}
	elem := s.list.PushBack(id)
	s.dict[id] = elem
	if s.iterator == nil {
		s.iterator = elem
	}
}

func (s *tiplist) RemoveTip(id message.Id) {
	s.mu.Lock()
	defer s.mu.Unlock()

	elem, contains := s.dict[id]
	if !contains {
		return
	}
	delete(s.dict, id)
	s.list.Remove(elem)
	if s.iterator == elem {
		s.next(elem)
	}
}

func (s *tiplist) Next() message.Id {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.iterator == nil {
		return message.EmptyId
	}
	id := s.iterator.Value.(message.Id)
	s.next(s.iterator)
	return id
}

func (s *tiplist) next(elem *list.Element) {
	s.iterator = elem.Next()
	if s.iterator == nil {
		s.iterator = s.list.Front()
	}
}

func startTipBroadcaster(shutdownSignal <-chan struct{}) {
	defer log.Infof("Stopping %s ... done", tipsBroadcasterName)

	removeClosure := events.NewClosure(tips.RemoveTip)
	addClosure := events.NewClosure(tips.AddTip)

	// attach the tip list to the TipSelector
	tipSelector := messagelayer.TipSelector()
	tipSelector.Events.TipRemoved.Attach(removeClosure)
	defer tipSelector.Events.TipRemoved.Detach(removeClosure)
	tipSelector.Events.TipAdded.Attach(addClosure)
	defer tipSelector.Events.TipAdded.Detach(addClosure)

	log.Infof("%s started: interval=%v", tipsBroadcasterName, tipsBroadcasterInterval)
	timeutil.Ticker(broadcastNextOldestTip, tipsBroadcasterInterval, shutdownSignal)
	log.Infof("Stopping %s ...", tipsBroadcasterName)
}

// broadcasts the next oldest tip from the tip pool to all connected neighbors.
func broadcastNextOldestTip() {
	msgID := tips.Next()
	if msgID == message.EmptyId {
		return
	}
	broadcastMessage(msgID)
}

// broadcasts the given message to all neighbors if it exists.
func broadcastMessage(msgID message.Id) {
	msgBytes, err := loadMessage(msgID)
	if err != nil {
		return
	}
	log.Debugw("broadcast tip", "id", msgID)
	Manager().SendMessage(msgBytes)
}
