package markers

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/database"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
)

func TestManagerConvergence(t *testing.T) {
	db := mapdb.NewMapDB()
	manager := NewManager(db, database.NewCacheTimeProvider(0))

	structureDetails1 := manager.InheritStructureDetails(nil, alwaysIncreaseIndex)
	assert.True(t, structureDetails1.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails2 := manager.InheritStructureDetails(nil, alwaysIncreaseIndex)
	assert.True(t, structureDetails2.PastMarkers.Equals(NewMarkers(NewMarker(0, 0))))

	structureDetails3 := manager.InheritStructureDetails(nil, alwaysIncreaseIndex)
	assert.True(t, structureDetails3.PastMarkers.Equals(NewMarkers(NewMarker(0, 0))))

	structureDetails4 := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails2}, alwaysIncreaseIndex)
	assert.True(t, structureDetails4.PastMarkers.Equals(NewMarkers(NewMarker(0, 2))))

	structureDetails5 := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails3}, alwaysIncreaseIndex)
	assert.True(t, structureDetails5.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails6 := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails2, structureDetails3}, alwaysIncreaseIndex)
	assert.True(t, structureDetails6.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails7 := manager.InheritStructureDetails([]*StructureDetails{structureDetails2, structureDetails3}, alwaysIncreaseIndex)
	assert.True(t, structureDetails7.PastMarkers.Equals(NewMarkers(NewMarker(0, 0))))

	structureDetails8 := manager.InheritStructureDetails([]*StructureDetails{structureDetails4, structureDetails5}, alwaysIncreaseIndex)
	assert.True(t, structureDetails8.PastMarkers.Equals(NewMarkers(NewMarker(0, 3))))

	structureDetails9 := manager.InheritStructureDetails([]*StructureDetails{structureDetails5, structureDetails6}, alwaysIncreaseIndex)
	assert.True(t, structureDetails9.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails10 := manager.InheritStructureDetails([]*StructureDetails{structureDetails6, structureDetails7}, alwaysIncreaseIndex)
	assert.True(t, structureDetails10.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails11 := manager.InheritStructureDetails([]*StructureDetails{structureDetails9, structureDetails10}, alwaysIncreaseIndex)
	assert.True(t, structureDetails11.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))
}

func alwaysIncreaseIndex(SequenceID, Index) bool {
	return true
}

func TestManager(t *testing.T) {
	testMessages := []*message{
		newMessage("msg1"),
		newMessage("msg2"),
		newMessage("msg3", "msg1", "msg2"),
		newMessage("msg4", "msg3"),
		newMessage("msg5", "msg1", "msg2"),
		newMessage("msg6", "msg3", "msg5"),
		newMessage("msg7", "msg4"),
		newMessage("msg8", "msg4"),
		newMessage("msg9", "msg8"),
		newMessage("msg10", "msg4"),
		newMessage("msg11", "msg6"),
		newMessage("msg12", "msg6"),
		newMessage("msg13", "msg6"),
	}

	messageDB := makeMessageDB(testMessages...)
	manager := NewManager(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))

	for _, m := range testMessages {
		if futureMarkerToPropagate := inheritPastMarkers(m, manager, messageDB); futureMarkerToPropagate != nil {
			//distributeNewFutureMarkerToPastCone(futureMarkerToPropagate, m.parents, manager, messageDB)
		}
	}

	for messageID, expectedParentMarkersMap := range map[string]map[SequenceID]Index{
		"msg1":  {0: 1},
		"msg2":  {0: 0},
		"msg3":  {0: 2},
		"msg4":  {0: 3},
		"msg5":  {0: 1},
		"msg6":  {0: 2},
		"msg7":  {0: 4},
		"msg8":  {1: 4},
		"msg9":  {1: 5},
		"msg10": {0: 3},
		"msg11": {2: 3},
		"msg12": {0: 2},
		"msg13": {3: 3},
	} {
		expectedPastMarkers := NewMarkers()
		for seq, index := range expectedParentMarkersMap {
			expectedPastMarkers.Set(seq, index)
		}

		assert.Equal(t, expectedPastMarkers, messageDB[messageID].structureDetails.PastMarkers, messageID+" has unexpected past Markers")
	}

	//for messageID, expectedFutureMarkers := range map[string]*Markers{
	//	"msg0":  NewMarkers(&Marker{sequenceID: 1, index: 2}, &Marker{sequenceID: 5, index: 2}),
	//	"msg1":  NewMarkers(&Marker{sequenceID: 1, index: 2}, &Marker{sequenceID: 5, index: 2}),
	//	"msg2":  NewMarkers(&Marker{sequenceID: 1, index: 2}),
	//	"msg3":  NewMarkers(&Marker{sequenceID: 1, index: 2}, &Marker{sequenceID: 5, index: 2}),
	//	"msg4":  NewMarkers(&Marker{sequenceID: 3, index: 3}),
	//	"msg5":  NewMarkers(&Marker{sequenceID: 3, index: 3}, &Marker{sequenceID: 1, index: 3}),
	//	"msg6":  NewMarkers(&Marker{sequenceID: 3, index: 3}),
	//	"msg7":  NewMarkers(&Marker{sequenceID: 3, index: 4}),
	//	"msg8":  NewMarkers(&Marker{sequenceID: 1, index: 3}),
	//	"msg9":  NewMarkers(&Marker{sequenceID: 3, index: 4}),
	//	"msg10": NewMarkers(&Marker{sequenceID: 3, index: 4}),
	//	"msg11": NewMarkers(&Marker{sequenceID: 3, index: 4}, &Marker{sequenceID: 5, index: 4}),
	//	"msg12": NewMarkers(),
	//	"msg13": NewMarkers(&Marker{sequenceID: 5, index: 2}),
	//	"msg14": NewMarkers(&Marker{sequenceID: 5, index: 2}),
	//	"msg15": NewMarkers(&Marker{sequenceID: 5, index: 4}),
	//	"msg16": NewMarkers(),
	//} {
	//	assert.Equal(t, expectedFutureMarkers, messageDB[messageID].structureDetails.FutureMarkers, messageID+" has unexpected future Markers")
	//}

	//for _, earlierMessage := range messageDB {
	//	for _, laterMessage := range messageDB {
	//		if earlierMessage != laterMessage {
	//			switch messageReferencesMessage(laterMessage, earlierMessage, messageDB) {
	//			case types.True:
	//				referencesResult := manager.IsInPastCone(earlierMessage.structureDetails, laterMessage.structureDetails)
	//				assert.True(t, referencesResult == types.True || referencesResult == types.Maybe, earlierMessage.id+" should be in past cone of "+laterMessage.id)
	//			case types.False:
	//				referencesResult := manager.IsInPastCone(earlierMessage.structureDetails, laterMessage.structureDetails)
	//				assert.True(t, referencesResult == types.False || referencesResult == types.Maybe, earlierMessage.id+" shouldn't be in past cone of "+laterMessage.id)
	//			}
	//		}
	//	}
	//}
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
		pastMarkers[i] = messageDB[parentID].structureDetails
	}

	// inherit new past Markers
	fmt.Println(message.id, "++++++++++++++++++++++++++++++")
	message.structureDetails = manager.InheritStructureDetails(pastMarkers, alwaysIncreaseIndex)
	//if message.structureDetails.IsPastMarker {
	//	pastMarkerToPropagate = message.structureDetails.PastMarkers.Marker()
	//}

	return
}

func distributeNewFutureMarkerToPastCone(futureMarker *Marker, messageParents []string, manager *Manager, messageDB map[string]*message) {
	nextMessageParents := make([]string, 0)
	for _, parentID := range messageParents {
		parentMessage := messageDB[parentID]

		if _, inheritFutureMarkerFurther := manager.UpdateStructureDetails(parentMessage.structureDetails, futureMarker); inheritFutureMarkerFurther {
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
	id               string
	forceNewMarker   bool
	parents          []string
	structureDetails *StructureDetails
}

func newMessage(id string, parents ...string) *message {
	return &message{
		id:      id,
		parents: parents,
		structureDetails: &StructureDetails{
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
