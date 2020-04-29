package payload

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	// create variable for id
	sourceID, err := NewID("4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM")
	if err != nil {
		panic(err)
	}

	// read serialized id into both variables
	var restoredIDPointer ID
	restoredIDValue, _, err := IDFromBytes(sourceID.Bytes(), &restoredIDPointer)
	if err != nil {
		panic(err)
	}

	// check if both variables give the same result
	assert.Equal(t, sourceID, restoredIDValue)
	assert.Equal(t, sourceID, restoredIDPointer)
}
