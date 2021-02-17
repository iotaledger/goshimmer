package markers

import (
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
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
		newMessage("msg16", true, []string{"msg11", "msg15"}),
	}

	messageDB := makeMessageDB(testMessages...)
	manager := NewManager(mapdb.NewMapDB())

	for _, message := range testMessages {
		if futureMarkerToPropagate := inheritPastMarkers(message, manager, messageDB); futureMarkerToPropagate != nil {
			distributeNewFutureMarkerToPastCone(futureMarkerToPropagate, message.parents, manager, messageDB)
		}
	}

	for messageID, expectedParentMarkers := range map[string]*Markers{
		"msg0":  NewMarkers(&Marker{sequenceID: 1, index: 1}),
		"msg1":  NewMarkers(&Marker{sequenceID: 0, index: 0}),
		"msg2":  NewMarkers(&Marker{sequenceID: 0, index: 0}),
		"msg3":  NewMarkers(&Marker{sequenceID: 1, index: 1}),
		"msg4":  NewMarkers(&Marker{sequenceID: 2, index: 1}),
		"msg5":  NewMarkers(&Marker{sequenceID: 1, index: 2}),
		"msg6":  NewMarkers(&Marker{sequenceID: 2, index: 1}),
		"msg7":  NewMarkers(&Marker{sequenceID: 3, index: 3}),
		"msg8":  NewMarkers(&Marker{sequenceID: 1, index: 2}),
		"msg9":  NewMarkers(&Marker{sequenceID: 1, index: 2}, &Marker{sequenceID: 2, index: 1}),
		"msg10": NewMarkers(&Marker{sequenceID: 3, index: 3}),
		"msg11": NewMarkers(&Marker{sequenceID: 1, index: 3}),
		"msg12": NewMarkers(&Marker{sequenceID: 3, index: 4}),
		"msg13": NewMarkers(&Marker{sequenceID: 4, index: 1}),
		"msg14": NewMarkers(&Marker{sequenceID: 4, index: 1}),
		"msg15": NewMarkers(&Marker{sequenceID: 5, index: 2}),
		"msg16": NewMarkers(&Marker{sequenceID: 5, index: 4}),
	} {
		assert.Equal(t, expectedParentMarkers, messageDB[messageID].markers.PastMarkers, messageID+" has unexpected past Markers")
	}

	for messageID, expectedFutureMarkers := range map[string]*Markers{
		"msg0":  NewMarkers(&Marker{sequenceID: 1, index: 2}, &Marker{sequenceID: 5, index: 2}),
		"msg1":  NewMarkers(&Marker{sequenceID: 1, index: 2}, &Marker{sequenceID: 5, index: 2}),
		"msg2":  NewMarkers(&Marker{sequenceID: 1, index: 2}),
		"msg3":  NewMarkers(&Marker{sequenceID: 1, index: 2}, &Marker{sequenceID: 5, index: 2}),
		"msg4":  NewMarkers(&Marker{sequenceID: 3, index: 3}),
		"msg5":  NewMarkers(&Marker{sequenceID: 3, index: 3}, &Marker{sequenceID: 1, index: 3}),
		"msg6":  NewMarkers(&Marker{sequenceID: 3, index: 3}),
		"msg7":  NewMarkers(&Marker{sequenceID: 3, index: 4}),
		"msg8":  NewMarkers(&Marker{sequenceID: 1, index: 3}),
		"msg9":  NewMarkers(&Marker{sequenceID: 3, index: 4}),
		"msg10": NewMarkers(&Marker{sequenceID: 3, index: 4}),
		"msg11": NewMarkers(&Marker{sequenceID: 3, index: 4}, &Marker{sequenceID: 5, index: 4}),
		"msg12": NewMarkers(),
		"msg13": NewMarkers(&Marker{sequenceID: 5, index: 2}),
		"msg14": NewMarkers(&Marker{sequenceID: 5, index: 2}),
		"msg15": NewMarkers(&Marker{sequenceID: 5, index: 4}),
		"msg16": NewMarkers(),
	} {
		assert.Equal(t, expectedFutureMarkers, messageDB[messageID].markers.FutureMarkers, messageID+" has unexpected future Markers")
	}

	for _, earlierMessage := range messageDB {
		for _, laterMessage := range messageDB {
			if earlierMessage != laterMessage {
				switch messageReferencesMessage(laterMessage, earlierMessage, messageDB) {
				case types.True:
					referencesResult := manager.IsInPastCone(earlierMessage.markers, laterMessage.markers)
					assert.True(t, referencesResult == types.True || referencesResult == types.Maybe, earlierMessage.id+" should be in past cone of "+laterMessage.id)
				case types.False:
					referencesResult := manager.IsInPastCone(earlierMessage.markers, laterMessage.markers)
					assert.True(t, referencesResult == types.False || referencesResult == types.Maybe, earlierMessage.id+" shouldn't be in past cone of "+laterMessage.id)
				}
			}
		}
	}
}

func messageReferencesMessage(laterMessage *message, earlierMessage *message, messageDB map[string]*message) types.TriBool {
	for _, parentID := range laterMessage.parents {
		if parentID == earlierMessage.id {
			return types.True
		}

		switch messageReferencesMessage(messageDB[parentID], earlierMessage, messageDB) {
		case types.True:
			return types.True
		case types.Maybe:
			return types.Maybe
		}
	}

	return types.False
}

func inheritPastMarkers(message *message, manager *Manager, messageDB map[string]*message) (pastMarkerToPropagate *Marker) {
	// merge past Markers of referenced parents
	pastMarkers := make([]*StructureDetails, len(message.parents))
	for i, parentID := range message.parents {
		pastMarkers[i] = messageDB[parentID].markers
	}

	// inherit new past Markers
	message.markers, _ = manager.InheritStructureDetails(pastMarkers, increaseIndex(message), message.optionalNewSequenceAlias...)
	if message.markers.IsPastMarker {
		pastMarkerToPropagate = message.markers.PastMarkers.FirstMarker()
	}

	return
}

func distributeNewFutureMarkerToPastCone(futureMarker *Marker, messageParents []string, manager *Manager, messageDB map[string]*message) {
	nextMessageParents := make([]string, 0)
	for _, parentID := range messageParents {
		parentMessage := messageDB[parentID]

		if _, inheritFutureMarkerFurther := manager.UpdateStructureDetails(parentMessage.markers, futureMarker); inheritFutureMarkerFurther {
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
	markers                  *StructureDetails
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
		markers: &StructureDetails{
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

func increaseIndex(message *message) IncreaseIndexCallback {
	return func(sequenceID SequenceID, currentHighestIndex Index) bool {
		return message.forceNewMarker
	}
}
