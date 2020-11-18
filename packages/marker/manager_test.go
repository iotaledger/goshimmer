package marker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/stringify"
)

/*
func TestMarkersManager_InheritMarkers(t *testing.T) {

	manager := NewManager(mapdb.NewMapDB())
	inheritedMarkers, isNewMarker, _ := manager.InheritPastMarkers(manager.NormalizeMarkers(NewMarkers()))
	fmt.Println(inheritedMarkers, isNewMarker)
	normalizedMarkers1, normalizedSequences1 := manager.NormalizeMarkers(NewMarkers())
	fmt.Println(manager.InheritPastMarkers(normalizedMarkers1, normalizedSequences1, NewSequenceAlias([]byte("testBranch"))))
	fmt.Println(manager.InheritPastMarkers(manager.NormalizeMarkers(inheritedMarkers)))
}

func Test(t *testing.T) {
	manager := NewManager(mapdb.NewMapDB())

	manager.sequenceStore.Store(NewSequence(0, NewMarkers(), 0))
	manager.sequenceStore.Store(NewSequence(1, NewMarkers(
		&Marker{0, 1},
	), 1))
	manager.sequenceStore.Store(NewSequence(2, NewMarkers(
		&Marker{0, 7},
	), 1))
	manager.sequenceStore.Store(NewSequence(3, NewMarkers(
		&Marker{1, 6},
		&Marker{2, 1},
	), 2))

	normalizedMarkers, normalizedSequences := manager.NormalizeMarkers(
		manager.MergeMarkers(
			NewMarkers(
				&Marker{1, 7},
			), NewMarkers(
				&Marker{0, 3},
				&Marker{2, 9},
			), NewMarkers(
				&Marker{3, 7},
			),
		),
	)

	fmt.Println(normalizedMarkers, normalizedSequences)
}
*/

func TestManager_InheritPastMarkers(t *testing.T) {
	messageDB := makeMessageDB(
		newMessage("msg0", false, nil),
		newMessage("msg1", false, nil),
		newMessage("msg2", false, nil, "msg1"),
		newMessage("msg3", false, nil, "msg0", "msg1"),
		newMessage("msg4", false, []byte("newSequence1")),
		newMessage("msg5", true, nil, "msg2", "msg3"),
		newMessage("msg6", false, nil, "msg4"),
		newMessage("msg7", false, nil, "msg5", "msg6"),
		newMessage("msg8", false, nil, "msg3", "msg5"),
		newMessage("msg9", false, nil, "msg5", "msg6"),
		newMessage("msg10", false, nil, "msg7", "msg9"),
		newMessage("msg11", true, nil, "msg8"),
		newMessage("msg12", true, nil, "msg6", "msg10", "msg11"),
		newMessage("msg13", false, []byte("newSequence2")),
		newMessage("msg14", false, nil, "msg13"),
		newMessage("msg15", false, nil, "msg3", "msg14"),
	)

	manager := NewManager(mapdb.NewMapDB())

	for i := 0; i < len(messageDB); i++ {
		messageID := fmt.Sprintf("msg%d", i)
		message := messageDB[messageID]

		parentMarkers := make([]Markers, len(message.parents))
		for i, parentID := range message.parents {
			parentMarkers[i] = messageDB[parentID].markers.PastMarkers
		}

		mergedMarkers := manager.MergeMarkers(parentMarkers...)
		normalizedMarkers, normalizedSequences := manager.NormalizeMarkers(mergedMarkers)

		switch message.newSequenceID {
		case nil:
			inheritedMarkers, isNewSequence, isNewMarker := manager.InheritPastMarkers(normalizedMarkers, normalizedSequences, func(sequenceID SequenceID, currentHighestIndex Index) bool {
				return message.forceNewMarker
			})

			message.markers.PastMarkers = inheritedMarkers

			fmt.Println(message.id, inheritedMarkers, isNewSequence, isNewMarker)
		default:
			inheritedMarkers, isNewSequence, isNewMarker := manager.InheritPastMarkers(normalizedMarkers, normalizedSequences, func(sequenceID SequenceID, currentHighestIndex Index) bool {
				return message.forceNewMarker
			}, NewSequenceAlias(message.newSequenceID))

			message.markers.PastMarkers = inheritedMarkers

			fmt.Println(message.id, inheritedMarkers, isNewSequence, isNewMarker)
		}
	}
}

func makeMessageDB(messages ...*message) (messageDB map[string]*message) {
	messageDB = make(map[string]*message)
	for _, msg := range messages {
		messageDB[msg.id] = msg
	}

	return
}

type message struct {
	id             string
	forceNewMarker bool
	newSequenceID  []byte
	parents        []string
	markers        *MessageMarkers
}

func newMessage(id string, forceNewMarker bool, newSequenceID []byte, parents ...string) *message {
	return &message{
		id:             id,
		forceNewMarker: forceNewMarker,
		newSequenceID:  newSequenceID,
		parents:        parents,
		markers:        &MessageMarkers{},
	}
}

func (m *message) String() string {
	return stringify.Struct("message",
		stringify.StructField("id", m.id),
		stringify.StructField("forceNewMarker", m.forceNewMarker),
		stringify.StructField("newSequenceID", m.newSequenceID),
		stringify.StructField("parents", m.parents),
	)
}
