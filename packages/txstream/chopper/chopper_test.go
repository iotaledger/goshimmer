// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0
package chopper_test

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/iotaledger/goshimmer/packages/txstream/chopper"

	"github.com/stretchr/testify/assert"
)

const choppingOverhead = 3

//goland:noinspection ALL
func testParameters(t *testing.T, maxMsgSize, dataLen int, expectChopped bool, expectNumChunks int) {
	c := chopper.NewChopper()

	data := make([]byte, dataLen)
	rand.Read(data)

	chopped, ok, err := c.ChopData(data, maxMsgSize, choppingOverhead)
	assert.NoError(t, err)
	assert.EqualValues(t, ok, expectChopped)
	if !ok {
		return
	}
	assert.Equal(t, expectNumChunks, len(chopped))
	assert.True(t, checkChunkLengths(chopped, maxMsgSize))
	for _, piece := range chopped {
		ret, err := c.IncomingChunk(piece, maxMsgSize, choppingOverhead)
		assert.NoError(t, err)
		if ret != nil {
			assert.True(t, bytes.Equal(data, ret))
		}
	}
}

func checkChunkLengths(chopped [][]byte, maxChunkSize int) bool {
	for _, d := range chopped {
		if len(d) > maxChunkSize {
			return false
		}
	}
	return true
}

func Test16K(t *testing.T) {
	maxMsgSize := 16 * 1024
	testParameters(t, maxMsgSize, 100, false, -1)
	testParameters(t, maxMsgSize, 200, false, -1)
	testParameters(t, maxMsgSize, 2000, false, -1)
	num, _, _ := chopper.NumChunks(30000, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 30000, true, int(num))
	num, _, _ = chopper.NumChunks(1000000, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 1000000, true, int(num))
	testParameters(t, maxMsgSize, maxMsgSize-2, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize-1, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize+1, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+2, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+3, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+4, true, 2)
	testParameters(t, maxMsgSize, 2*maxMsgSize, true, 3)
	testParameters(t, maxMsgSize, 3*maxMsgSize, true, 4)
	num, _, _ = chopper.NumChunks(3*maxMsgSize, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 3*maxMsgSize, true, int(num))
}

func Test500B(t *testing.T) {
	maxMsgSize := 500
	testParameters(t, maxMsgSize, 100, false, -1)
	testParameters(t, maxMsgSize, 200, false, -1)
	testParameters(t, maxMsgSize, 2000, true, 5)
	num, _, _ := chopper.NumChunks(30000, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 30000, true, int(num))
	num, _, _ = chopper.NumChunks(100000, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 100000, true, int(num))
	testParameters(t, maxMsgSize, maxMsgSize-2, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize-1, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize+1, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+2, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+3, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+4, true, 2)
	testParameters(t, maxMsgSize, 2*maxMsgSize, true, 3)
	testParameters(t, maxMsgSize, 3*maxMsgSize, true, 4)
	num, _, _ = chopper.NumChunks(3*maxMsgSize, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 3*maxMsgSize, true, int(num))
}

func TestMaxTangleMessage(t *testing.T) {
	maxMsgSize := 64 * 1024
	testParameters(t, maxMsgSize, 100, false, -1)
	testParameters(t, maxMsgSize, 200, false, -1)
	testParameters(t, maxMsgSize, 2000, false, -1)
	num, _, _ := chopper.NumChunks(66000, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 66000, true, int(num))
	num, _, _ = chopper.NumChunks(1000000, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 1000000, true, int(num))
	testParameters(t, maxMsgSize, maxMsgSize-2, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize-1, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize, false, -1)
	testParameters(t, maxMsgSize, maxMsgSize+1, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+2, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+3, true, 2)
	testParameters(t, maxMsgSize, maxMsgSize+4, true, 2)
	testParameters(t, maxMsgSize, 2*maxMsgSize, true, 3)
	testParameters(t, maxMsgSize, 3*maxMsgSize, true, 4)
	num, _, _ = chopper.NumChunks(3*maxMsgSize, maxMsgSize, choppingOverhead)
	testParameters(t, maxMsgSize, 3*maxMsgSize, true, int(num))
}

//goland:noinspection ALL
func TestTooLongData(t *testing.T) {
	short := 10000
	chunk := 500
	tooLong := (chunk * chopper.MaxNChunks)

	assert.True(t, tooLong/chunk >= chopper.MaxNChunks)

	dataLong := make([]byte, tooLong)
	rand.Read(dataLong)
	c := chopper.NewChopper()
	_, _, err := c.ChopData(dataLong, 500, choppingOverhead)
	assert.Error(t, err)

	dataShort := make([]byte, short)
	rand.Read(dataShort)
	_, _, err = c.ChopData(dataShort, 500, choppingOverhead)
	assert.NoError(t, err)
}
