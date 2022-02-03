package markers

import (
	"testing"

	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
)

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
		newMessage("msg12", "msg11"),
		newMessage("msg13", "msg12"),
		newMessage("msg14", "msg11"),
		newMessage("msg15", "msg13", "msg14"),
		newMessage("msg16", "msg9", "msg15"),
		newMessage("msg17", "msg7", "msg16"),
		newMessage("msg18", "msg7", "msg13", "msg14"),
		newMessage("msg19", "msg7", "msg13", "msg14"),
		newMessage("msg20", "msg7", "msg13", "msg14", "msg11"),
		newMessage("msg21", "msg20"),
		newMessage("msg22", "msg21"),
	}

	messageDB := makeMessageDB(testMessages...)
	manager := NewManager(WithCacheTime(0), WithMaxPastMarkerDistance(3))

	for _, m := range testMessages {
		if futureMarkerToPropagate := inheritPastMarkers(m, manager, messageDB); futureMarkerToPropagate != nil {
			distributeNewFutureMarkerToPastCone(futureMarkerToPropagate, m.parents, manager, messageDB)
		}
	}

	type expectedStructureDetailsType struct {
		PastMarkers        *Markers
		PastMarkersGap     uint64
		ReferencedMarkers  *Markers
		ReferencingMarkers *Markers
		FutureMarkers      *Markers
	}

	expectedStructureDetails := map[string]expectedStructureDetailsType{
		"msg1": {
			PastMarkers: NewMarkers(
				NewMarker(0, 1),
			),
			PastMarkersGap:    0,
			ReferencedMarkers: NewMarkers(),
			ReferencingMarkers: NewMarkers(
				NewMarker(1, 3),
				NewMarker(2, 3),
				NewMarker(3, 5),
			),
			FutureMarkers: NewMarkers(
				NewMarker(0, 2),
			),
		},
		"msg2": {
			PastMarkers: NewMarkers(
				NewMarker(0, 0),
			),
			PastMarkersGap:    0,
			ReferencedMarkers: NewMarkers(),
			ReferencingMarkers: NewMarkers(
				NewMarker(1, 3),
				NewMarker(2, 3),
				NewMarker(3, 5),
			),
			FutureMarkers: NewMarkers(
				NewMarker(0, 2),
			),
		},
		"msg3": {
			PastMarkers: NewMarkers(
				NewMarker(0, 2),
			),
			PastMarkersGap:    0,
			ReferencedMarkers: NewMarkers(),
			ReferencingMarkers: NewMarkers(
				NewMarker(1, 3),
				NewMarker(2, 3),
				NewMarker(3, 5),
			),
			FutureMarkers: NewMarkers(
				NewMarker(0, 3),
				NewMarker(1, 3),
				NewMarker(2, 3),
			),
		},
		"msg4": {
			PastMarkers: NewMarkers(
				NewMarker(0, 3),
			),
			PastMarkersGap:    0,
			ReferencedMarkers: NewMarkers(),
			ReferencingMarkers: NewMarkers(
				NewMarker(1, 5),
				NewMarker(2, 6),
				NewMarker(3, 5),
			),
			FutureMarkers: NewMarkers(
				NewMarker(0, 4),
				NewMarker(2, 6),
			),
		},
		"msg5": {
			PastMarkers: NewMarkers(
				NewMarker(0, 1),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(1, 3),
				NewMarker(2, 3),
			),
		},
		"msg6": {
			PastMarkers: NewMarkers(
				NewMarker(0, 2),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(1, 3),
				NewMarker(2, 3),
			),
		},
		"msg7": {
			PastMarkers: NewMarkers(
				NewMarker(0, 4),
			),
			PastMarkersGap:    0,
			ReferencedMarkers: NewMarkers(),
			ReferencingMarkers: NewMarkers(
				NewMarker(1, 5),
				NewMarker(2, 7),
				NewMarker(3, 5),
			),
			FutureMarkers: NewMarkers(
				NewMarker(0, 5),
				NewMarker(1, 5),
				NewMarker(2, 7),
				NewMarker(3, 5),
			),
		},
		"msg8": {
			PastMarkers: NewMarkers(
				NewMarker(0, 3),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(2, 6),
			),
		},
		"msg9": {
			PastMarkers: NewMarkers(
				NewMarker(0, 3),
			),
			PastMarkersGap:     2,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(2, 6),
			),
		},
		"msg10": {
			PastMarkers: NewMarkers(
				NewMarker(0, 3),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers:      NewMarkers(),
		},
		"msg11": {
			PastMarkers: NewMarkers(
				NewMarker(0, 2),
			),
			PastMarkersGap:     2,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(1, 3),
				NewMarker(2, 3),
			),
		},
		"msg12": {
			PastMarkers: NewMarkers(
				NewMarker(1, 3),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 2),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(2, 5),
				NewMarker(0, 5),
				NewMarker(3, 5),
			),
			FutureMarkers: NewMarkers(
				NewMarker(1, 4),
			),
		},
		"msg13": {
			PastMarkers: NewMarkers(
				NewMarker(1, 4),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 2),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(0, 5),
				NewMarker(2, 5),
				NewMarker(3, 5),
			),
			FutureMarkers: NewMarkers(
				NewMarker(0, 5),
				NewMarker(1, 5),
				NewMarker(2, 5),
				NewMarker(3, 5),
			),
		},
		"msg14": {
			PastMarkers: NewMarkers(
				NewMarker(2, 3),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 2),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(0, 5),
				NewMarker(1, 5),
				NewMarker(3, 5),
			),
			FutureMarkers: NewMarkers(
				NewMarker(0, 5),
				NewMarker(1, 5),
				NewMarker(2, 5),
				NewMarker(3, 5),
			),
		},
		"msg15": {
			PastMarkers: NewMarkers(
				NewMarker(2, 5),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 4),
				NewMarker(0, 2),
			),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(2, 6),
			),
		},
		"msg16": {
			PastMarkers: NewMarkers(
				NewMarker(2, 6),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 3),
				NewMarker(1, 4),
			),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(2, 7),
			),
		},
		"msg17": {
			PastMarkers: NewMarkers(
				NewMarker(2, 7),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 4),
				NewMarker(1, 4),
			),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers:      NewMarkers(),
		},
		"msg18": {
			PastMarkers: NewMarkers(
				NewMarker(1, 5),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 4),
				NewMarker(2, 3),
			),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers:      NewMarkers(),
		},
		"msg19": {
			PastMarkers: NewMarkers(
				NewMarker(0, 5),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 4),
				NewMarker(2, 3),
			),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers:      NewMarkers(),
		},
		"msg20": {
			PastMarkers: NewMarkers(
				NewMarker(0, 4),
				NewMarker(1, 4),
				NewMarker(2, 3),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(3, 5),
			),
		},
		"msg21": {
			PastMarkers: NewMarkers(
				NewMarker(0, 4),
				NewMarker(1, 4),
				NewMarker(2, 3),
			),
			PastMarkersGap:     2,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers: NewMarkers(
				NewMarker(3, 5),
			),
		},
		"msg22": {
			PastMarkers: NewMarkers(
				NewMarker(3, 5),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 4),
				NewMarker(1, 4),
				NewMarker(2, 3),
			),
			ReferencingMarkers: NewMarkers(),
			FutureMarkers:      NewMarkers(),
		},
	}

	for messageID, messageExpected := range expectedStructureDetails {
		assert.Equal(t, messageExpected.PastMarkers, messageDB[messageID].structureDetails.PastMarkers, messageID+" has unexpected past Markers")
		assert.Equal(t, messageExpected.FutureMarkers, messageDB[messageID].structureDetails.FutureMarkers, messageID+" has unexpected future Markers")
		assert.Equal(t, messageExpected.PastMarkersGap, messageDB[messageID].structureDetails.PastMarkerGap, messageID+" has unexpected PastMarkerGap")

		if messageExpected.PastMarkersGap == 0 {
			pastMarker := messageExpected.PastMarkers.Marker()

			manager.Sequence(pastMarker.SequenceID()).Consume(func(sequence *Sequence) {
				assert.Equal(t, messageExpected.ReferencedMarkers, sequence.ReferencedMarkers(pastMarker.Index()), messageID+" has unexpected referenced Markers")
				assert.Equal(t, messageExpected.ReferencingMarkers, sequence.ReferencingMarkers(pastMarker.Index()), messageID+" has unexpected referencing Markers")
			})
		}
	}

	for _, earlierMessage := range messageDB {
		for _, laterMessage := range messageDB {
			if earlierMessage != laterMessage {
				switch messageReferencesMessage(laterMessage, earlierMessage, messageDB) {
				case types.True:
					referencesResult := manager.IsInPastCone(earlierMessage.structureDetails, laterMessage.structureDetails)
					assert.True(t, referencesResult == types.True || referencesResult == types.Maybe, earlierMessage.id+" should be in past cone of "+laterMessage.id)
				case types.False:
					referencesResult := manager.IsInPastCone(earlierMessage.structureDetails, laterMessage.structureDetails)
					assert.True(t, referencesResult == types.False || referencesResult == types.Maybe, earlierMessage.id+" shouldn't be in past cone of "+laterMessage.id)
				}
			}
		}
	}
}

func TestManagerConvergence(t *testing.T) {
	manager := NewManager(WithCacheTime(0))

	structureDetails1, _ := manager.InheritStructureDetails(nil, alwaysIncreaseIndex)
	assert.True(t, structureDetails1.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails2, _ := manager.InheritStructureDetails(nil, alwaysIncreaseIndex)
	assert.True(t, structureDetails2.PastMarkers.Equals(NewMarkers(NewMarker(0, 0))))

	structureDetails3, _ := manager.InheritStructureDetails(nil, alwaysIncreaseIndex)
	assert.True(t, structureDetails3.PastMarkers.Equals(NewMarkers(NewMarker(0, 0))))

	structureDetails4, _ := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails2}, alwaysIncreaseIndex)
	assert.True(t, structureDetails4.PastMarkers.Equals(NewMarkers(NewMarker(0, 2))))

	structureDetails5, _ := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails3}, alwaysIncreaseIndex)
	assert.True(t, structureDetails5.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails6, _ := manager.InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails2, structureDetails3}, alwaysIncreaseIndex)
	assert.True(t, structureDetails6.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails7, _ := manager.InheritStructureDetails([]*StructureDetails{structureDetails2, structureDetails3}, alwaysIncreaseIndex)
	assert.True(t, structureDetails7.PastMarkers.Equals(NewMarkers(NewMarker(0, 0))))

	structureDetails8, _ := manager.InheritStructureDetails([]*StructureDetails{structureDetails4, structureDetails5}, alwaysIncreaseIndex)
	assert.True(t, structureDetails8.PastMarkers.Equals(NewMarkers(NewMarker(0, 3))))

	structureDetails9, _ := manager.InheritStructureDetails([]*StructureDetails{structureDetails5, structureDetails6}, alwaysIncreaseIndex)
	assert.True(t, structureDetails9.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails10, _ := manager.InheritStructureDetails([]*StructureDetails{structureDetails6, structureDetails7}, alwaysIncreaseIndex)
	assert.True(t, structureDetails10.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails11, _ := manager.InheritStructureDetails([]*StructureDetails{structureDetails9, structureDetails10}, alwaysIncreaseIndex)
	assert.True(t, structureDetails11.PastMarkers.Equals(NewMarkers(NewMarker(0, 1))))
}

func alwaysIncreaseIndex(SequenceID, Index) bool {
	return true
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

	message.structureDetails, _ = manager.InheritStructureDetails(pastMarkers, alwaysIncreaseIndex)
	if message.structureDetails.IsPastMarker {
		pastMarkerToPropagate = message.structureDetails.PastMarkers.Marker()
	}

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
