package markers

import (
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
)

func TestManagerConvergence(t *testing.T) {
	db := mapdb.NewMapDB()
	manager := NewManager(db)

	structureDetails1, newSequenceCreated1 := manager.InheritStructureDetails(nil, alwaysIncreaseIndex, NewSequenceAlias([]byte("1")))
	assert.True(t, structureDetails1.PastMarkers.Equals(NewMarkers(NewMarker(1, 1))))
	assert.True(t, newSequenceCreated1)

	structureDetails2, newSequenceCreated2 := manager.InheritStructureDetails(nil, alwaysIncreaseIndex, NewSequenceAlias([]byte("2")))
	assert.True(t, structureDetails2.PastMarkers.Equals(NewMarkers(NewMarker(2, 1))))
	assert.True(t, newSequenceCreated2)

	structureDetails3, newSequenceCreated3 := manager.InheritStructureDetails(nil, alwaysIncreaseIndex, NewSequenceAlias([]byte("3")))
	assert.True(t, structureDetails3.PastMarkers.Equals(NewMarkers(NewMarker(3, 1))))
	assert.True(t, newSequenceCreated3)

	structureDetails4, newSequenceCreated4 := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails2}, alwaysIncreaseIndex, NewSequenceAlias([]byte("1+2")))
	assert.True(t, structureDetails4.PastMarkers.Equals(NewMarkers(NewMarker(4, 2))))
	assert.True(t, newSequenceCreated4)

	structureDetails5, newSequenceCreated5 := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails3}, alwaysIncreaseIndex, NewSequenceAlias([]byte("1+3")))
	assert.True(t, structureDetails5.PastMarkers.Equals(NewMarkers(NewMarker(5, 2))))
	assert.True(t, newSequenceCreated5)

	structureDetails6, newSequenceCreated6 := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails2, structureDetails3}, alwaysIncreaseIndex, NewSequenceAlias([]byte("1+2+3")))
	assert.True(t, structureDetails6.PastMarkers.Equals(NewMarkers(NewMarker(6, 2))))
	assert.True(t, newSequenceCreated6)

	structureDetails7, newSequenceCreated7 := manager.InheritStructureDetails([]*StructureDetails{structureDetails2, structureDetails3}, alwaysIncreaseIndex, NewSequenceAlias([]byte("2+3")))
	assert.True(t, structureDetails7.PastMarkers.Equals(NewMarkers(NewMarker(7, 2))))
	assert.True(t, newSequenceCreated7)

	structureDetails8, newSequenceCreated8 := manager.InheritStructureDetails([]*StructureDetails{structureDetails4, structureDetails5}, alwaysIncreaseIndex, NewSequenceAlias([]byte("1+2+3")))
	assert.True(t, structureDetails8.PastMarkers.Equals(NewMarkers(NewMarker(4, 2), NewMarker(5, 2))))
	assert.False(t, newSequenceCreated8)

	structureDetails9, newSequenceCreated9 := manager.InheritStructureDetails([]*StructureDetails{structureDetails5, structureDetails6}, alwaysIncreaseIndex, NewSequenceAlias([]byte("1+2+3")))
	assert.True(t, structureDetails9.PastMarkers.Equals(NewMarkers(NewMarker(6, 3))))
	assert.False(t, newSequenceCreated9)

	structureDetails10, newSequenceCreated10 := manager.InheritStructureDetails([]*StructureDetails{structureDetails6, structureDetails7}, alwaysIncreaseIndex, NewSequenceAlias([]byte("1+2+3")))
	assert.True(t, structureDetails10.PastMarkers.Equals(NewMarkers(NewMarker(6, 2), NewMarker(7, 2))))
	assert.False(t, newSequenceCreated10)

	structureDetails11, newSequenceCreated11 := manager.InheritStructureDetails([]*StructureDetails{structureDetails9, structureDetails10}, alwaysIncreaseIndex, NewSequenceAlias([]byte("1+2+3")))
	assert.True(t, structureDetails11.PastMarkers.Equals(NewMarkers(NewMarker(6, 4))))
	assert.False(t, newSequenceCreated11)
}

func alwaysIncreaseIndex(SequenceID, Index) bool {
	return true
}

func TestManager(t *testing.T) {
	testMessages := []*message{
		newMessage("msg0", false, nil, "sequence0"),
		newMessage("msg1", false, nil, "sequence0"),
		newMessage("msg2", false, []string{"msg1"}, "sequence0"),
		newMessage("msg3", false, []string{"msg0", "msg1"}, "sequence0"),
		newMessage("msg4", false, nil, "sequence1"),
		newMessage("msg5", true, []string{"msg2", "msg3"}, "sequence0"),
		newMessage("msg6", false, []string{"msg4"}, "sequence1"),
		newMessage("msg7", false, []string{"msg5", "msg6"}, "sequence2"),
		newMessage("msg8", false, []string{"msg3", "msg5"}, "sequence0"),
		newMessage("msg9", false, []string{"msg5", "msg6"}, "sequence2"),
		newMessage("msg10", false, []string{"msg7", "msg9"}, "sequence2"),
		newMessage("msg11", true, []string{"msg8"}, "sequence0"),
		newMessage("msg12", true, []string{"msg2", "msg6", "msg10", "msg11"}, "sequence2"),
		newMessage("msg13", false, nil, "sequence3"),
		newMessage("msg14", false, []string{"msg13"}, "sequence3"),
		newMessage("msg15", false, []string{"msg3", "msg14"}, "sequence4"),
		newMessage("msg16", true, []string{"msg11", "msg15"}, "sequence4"),
	}

	messageDB := makeMessageDB(testMessages...)
	db := mapdb.NewMapDB()
	manager := NewManager(db)

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

func messageReferencesMessage(laterMessage, earlierMessage *message, messageDB map[string]*message) types.TriBool {
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
	message.markers, _ = manager.InheritStructureDetails(pastMarkers, increaseIndex(message), message.sequenceAlias)
	if message.markers.IsPastMarker {
		pastMarkerToPropagate = message.markers.PastMarkers.Marker()
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
	id             string
	forceNewMarker bool
	parents        []string
	sequenceAlias  SequenceAlias
	markers        *StructureDetails
}

func newMessage(id string, forceNewMarker bool, parents []string, sequenceAlias string) *message {
	return &message{
		id:             id,
		forceNewMarker: forceNewMarker,
		sequenceAlias:  NewSequenceAlias([]byte(sequenceAlias)),
		parents:        parents,
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
