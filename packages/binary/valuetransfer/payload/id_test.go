package payload

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	// create variable for id
	sourceId, err := NewId("4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM")
	if err != nil {
		panic(err)
	}

	// read serialized id into both variables
	var restoredIdPointer Id
	restoredIdValue, err, _ := IdFromBytes(sourceId.Bytes(), &restoredIdPointer)
	if err != nil {
		panic(err)
	}

	// check if both variables give the same result
	assert.Equal(t, sourceId, restoredIdValue)
	assert.Equal(t, sourceId, restoredIdPointer)
}
