package marker

import (
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/stretchr/testify/assert"
)

func TestManager(t *testing.T) {
	testMessages := []*message{
		newMessage("msg0", false, nil),
		newMessage("msg1", false, nil),
		newMessage("msg2", false, []string{"msg1"}),
		newMessage("msg3", false, []string{"msg0", "msg1"}),
		newMessage("msg4", false, nil, "newSequence1"),
		newMessage("msg5", true, []string{"msg2", "msg3"}),
		newMessage("msg6", false, []string{"msg4"}),
		newMessage("msg7", false, []string{"msg5", "msg6"}),
		newMessage("msg8", false, []string{"msg3", "msg5"}),
		newMessage("msg9", false, []string{"msg5", "msg6"}),
		newMessage("msg10", false, []string{"msg7", "msg9"}),
		newMessage("msg11", true, []string{"msg8"}),
		newMessage("msg12", true, []string{"msg2", "msg6", "msg10", "msg11"}),
		newMessage("msg13", false, nil, "newSequence2"),
		newMessage("msg14", false, []string{"msg13"}),
		newMessage("msg15", false, []string{"msg3", "msg14"}),
	}

	messageDB := makeMessageDB(testMessages...)
	manager := NewManager(mapdb.NewMapDB())

	for _, message := range testMessages {
		if futureMarkerToPropagate := inheritPastMarkers(message, manager, messageDB); futureMarkerToPropagate != nil {
			distributeNewFutureMarkerToPastCone(futureMarkerToPropagate, message.parents, manager, messageDB)
		}
	}

	for messageID, expectedParentMarkers := range map[string]*Markers{
		"msg0":  NewMarkers(&Marker{sequenceID: 0, index: 1}),
		"msg1":  NewMarkers(),
		"msg2":  NewMarkers(),
		"msg3":  NewMarkers(&Marker{sequenceID: 0, index: 1}),
		"msg4":  NewMarkers(&Marker{sequenceID: 1, index: 1}),
		"msg5":  NewMarkers(&Marker{sequenceID: 0, index: 2}),
		"msg6":  NewMarkers(&Marker{sequenceID: 1, index: 1}),
		"msg7":  NewMarkers(&Marker{sequenceID: 2, index: 3}),
		"msg8":  NewMarkers(&Marker{sequenceID: 0, index: 2}),
		"msg9":  NewMarkers(&Marker{sequenceID: 0, index: 2}, &Marker{sequenceID: 1, index: 1}),
		"msg10": NewMarkers(&Marker{sequenceID: 2, index: 3}),
		"msg11": NewMarkers(&Marker{sequenceID: 0, index: 3}),
		"msg12": NewMarkers(&Marker{sequenceID: 2, index: 4}),
		"msg13": NewMarkers(&Marker{sequenceID: 3, index: 1}),
		"msg14": NewMarkers(&Marker{sequenceID: 3, index: 1}),
		"msg15": NewMarkers(&Marker{sequenceID: 4, index: 2}),
	} {
		assert.Equal(t, expectedParentMarkers, messageDB[messageID].markers.PastMarkers, messageID+" has unexpected past Markers")
	}

	for messageID, expectedFutureMarkers := range map[string]*Markers{
		"msg0":  NewMarkers(&Marker{sequenceID: 0, index: 2}, &Marker{sequenceID: 4, index: 2}),
		"msg1":  NewMarkers(&Marker{sequenceID: 0, index: 2}, &Marker{sequenceID: 4, index: 2}),
		"msg2":  NewMarkers(&Marker{sequenceID: 0, index: 2}),
		"msg3":  NewMarkers(&Marker{sequenceID: 0, index: 2}, &Marker{sequenceID: 4, index: 2}),
		"msg4":  NewMarkers(&Marker{sequenceID: 2, index: 3}),
		"msg5":  NewMarkers(&Marker{sequenceID: 2, index: 3}, &Marker{sequenceID: 0, index: 3}),
		"msg6":  NewMarkers(&Marker{sequenceID: 2, index: 3}),
		"msg7":  NewMarkers(&Marker{sequenceID: 2, index: 4}),
		"msg8":  NewMarkers(&Marker{sequenceID: 0, index: 3}),
		"msg9":  NewMarkers(&Marker{sequenceID: 2, index: 4}),
		"msg10": NewMarkers(&Marker{sequenceID: 2, index: 4}),
		"msg11": NewMarkers(&Marker{sequenceID: 2, index: 4}),
		"msg12": NewMarkers(),
		"msg13": NewMarkers(&Marker{sequenceID: 4, index: 2}),
		"msg14": NewMarkers(&Marker{sequenceID: 4, index: 2}),
		"msg15": NewMarkers(),
	} {
		assert.Equal(t, expectedFutureMarkers, messageDB[messageID].markers.FutureMarkers, messageID+" has unexpected future Markers")
	}
}

func inheritPastMarkers(message *message, manager *Manager, messageDB map[string]*message) (pastMarkerToPropagate *Marker) {
	// merge past Markers of referenced parents
	pastMarkers := NewMarkers()
	for _, parentID := range message.parents {
		pastMarkers.Merge(messageDB[parentID].markers.PastMarkers)
	}

	// inherit new past Markers
	newPastMarkers, _, pastMarkerToPropagate := manager.InheritPastMarkers(pastMarkers, increaseIndex(message), message.optionalNewSequenceAlias...)
	message.markers.IsPastMarker = pastMarkerToPropagate != nil
	message.markers.PastMarkers = newPastMarkers

	return
}

func distributeNewFutureMarkerToPastCone(futureMarker *Marker, messageParents []string, manager *Manager, messageDB map[string]*message) {
	nextMessageParents := make([]string, 0)
	for _, parentID := range messageParents {
		parentMessage := messageDB[parentID]

		newFutureMarkers, futureMarkersUpdated, inheritFutureMarkerFurther := manager.InheritFutureMarkers(parentMessage.markers.FutureMarkers, futureMarker, parentMessage.markers.IsPastMarker)
		if futureMarkersUpdated {
			parentMessage.markers.FutureMarkers = newFutureMarkers
		}

		if inheritFutureMarkerFurther {
			nextMessageParents = append(nextMessageParents, parentMessage.parents...)
		}
	}

	if len(nextMessageParents) >= 1 {
		distributeNewFutureMarkerToPastCone(futureMarker, nextMessageParents, manager, messageDB)
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
	id                       string
	forceNewMarker           bool
	parents                  []string
	optionalNewSequenceAlias []SequenceAlias
	markers                  *MarkersPair
}

func newMessage(id string, forceNewMarker bool, parents []string, optionalNewSequenceID ...string) *message {
	var optionalNewSequenceAlias []SequenceAlias
	if len(optionalNewSequenceID) >= 1 {
		optionalNewSequenceAlias = []SequenceAlias{NewSequenceAlias([]byte(optionalNewSequenceID[0]))}
	}

	return &message{
		id:                       id,
		forceNewMarker:           forceNewMarker,
		optionalNewSequenceAlias: optionalNewSequenceAlias,
		parents:                  parents,
		markers: &MarkersPair{
			PastMarkers:   NewMarkers(),
			FutureMarkers: NewMarkers(),
		},
	}
}

func (m *message) String() string {
	return stringify.Struct("message",
		stringify.StructField("id", m.id),
		stringify.StructField("forceNewMarker", m.forceNewMarker),
		stringify.StructField("parents", m.parents),
	)
}

func increaseIndex(message *message) IncreaseMarkerCallback {
	return func(sequenceID SequenceID, currentHighestIndex Index) bool {
		return message.forceNewMarker
	}
}
