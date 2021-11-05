// Package chopper helps splitting messages into smaller pieces and reassemble them
package chopper

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"bytes"
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	// for the final data packet to be not bigger than tangle.MaxMessageSize
	// 4 - chunk id, 2 seq nr, 2 num chunks, 2 - data len.
	chunkHeaderSize     = 4 + 2 + 2 + 2
	maxTTL              = 5 * time.Minute
	cleanupLoopInterval = 10 * time.Second
	// MaxNChunks maximum number of chunks.
	MaxNChunks = math.MaxUint16
)

// Chopper handles the splitting and joining of large messages.
type Chopper struct {
	nextID  uint32
	mutex   sync.Mutex
	chunks  map[uint32]*dataInProgress
	closeCh chan bool
}

type dataInProgress struct {
	buffer      [][]byte
	ttl         time.Time
	numReceived int
}

// NewChopper creates a new chopper instance.
func NewChopper() *Chopper {
	c := Chopper{
		nextID:  0,
		chunks:  make(map[uint32]*dataInProgress),
		closeCh: make(chan bool),
	}
	go c.cleanupLoop()
	return &c
}

// Close stops the internal cleanup goroutine.
func (c *Chopper) Close() {
	close(c.closeCh)
}

func (c *Chopper) cleanupLoop() {
	for {
		select {
		case <-c.closeCh:
			return
		case <-time.After(cleanupLoopInterval):
			toDelete := make([]uint32, 0)
			nowis := time.Now()
			c.mutex.Lock()
			for id, dip := range c.chunks {
				if nowis.After(dip.ttl) {
					toDelete = append(toDelete, id)
				}
			}
			for _, id := range toDelete {
				delete(c.chunks, id)
			}
			c.mutex.Unlock()
		}
	}
}

func (c *Chopper) getNextMsgID() uint32 {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.nextID++
	return c.nextID
}

// NumChunks returns the expected amount of chunks for the given data length.
func NumChunks(dataLen, maxMsgSize, includingChoppingOverhead int) (uint16, int, error) {
	if dataLen <= maxMsgSize {
		return 0, 0, nil // no need to split
	}
	maxChunkSize := maxMsgSize - includingChoppingOverhead
	maxSizeWithoutHeader := maxChunkSize - chunkHeaderSize
	if dataLen > maxChunkSize*MaxNChunks {
		return 0, 0, fmt.Errorf("chopper.NumChunks: too long data to chop")
	}
	numChunks := dataLen / maxSizeWithoutHeader
	if dataLen%maxSizeWithoutHeader > 0 {
		numChunks++
	}
	if numChunks < 2 { //nolint:gomnd
		panic("ChopData: internal inconsistency")
	}
	return uint16(numChunks), maxChunkSize, nil
}

// ChopData chops data into pieces (not more than 255) and adds chopper header to each piece
// for IncomingChunk function to reassemble it.
func (c *Chopper) ChopData(data []byte, maxMsgSize, includingChoppingOverhead int) ([][]byte, bool, error) {
	numChunks, maxChunkSize, err := NumChunks(len(data), maxMsgSize, includingChoppingOverhead)
	if err != nil {
		return nil, false, err
	}
	if numChunks == 0 {
		return nil, false, nil
	}
	maxSizeWithoutHeader := maxChunkSize - chunkHeaderSize
	id := c.getNextMsgID()
	ret := make([][]byte, 0, numChunks)
	var d []byte
	for i := uint16(0); i < numChunks; i++ {
		if len(data) > maxSizeWithoutHeader {
			d = data[:maxSizeWithoutHeader]
			data = data[maxSizeWithoutHeader:]
		} else {
			d = data
		}
		chunk := &msgChunk{
			msgID:       id,
			chunkSeqNum: i,
			numChunks:   numChunks,
			data:        d,
		}
		dtmp := chunk.encode()
		if len(dtmp) > maxMsgSize {
			panic("ChopData: internal inconsistency 2")
		}
		ret = append(ret, dtmp)
	}
	return ret, true, nil
}

// IncomingChunk collects all incoming chunks.
// Returned != nil value of the reassembled data
// maxChunkSize parameter must be the same on both sides.
func (c *Chopper) IncomingChunk(data []byte, maxMsgSize, includingChoppingOverhead int) ([]byte, error) {
	maxChunkSize := maxMsgSize - includingChoppingOverhead
	maxSizeWithoutHeader := maxChunkSize - chunkHeaderSize
	msg := msgChunk{}
	if err := msg.decode(data, maxSizeWithoutHeader); err != nil {
		return nil, err
	}
	switch {
	case len(msg.data) > maxChunkSize:
		return nil, fmt.Errorf("incomingChunk: too long data chunk")

	case msg.chunkSeqNum >= msg.numChunks:
		return nil, fmt.Errorf("incomingChunk: wrong incoming data chunk seq number")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	dip, ok := c.chunks[msg.msgID]
	if !ok {
		dip = &dataInProgress{
			buffer: make([][]byte, int(msg.numChunks)),
			ttl:    time.Now().Add(maxTTL),
		}
		c.chunks[msg.msgID] = dip
	} else if dip.buffer[msg.chunkSeqNum] != nil {
		return nil, fmt.Errorf("incomingChunk: repeating seq number")
	}
	dip.buffer[msg.chunkSeqNum] = msg.data
	dip.numReceived++

	if dip.numReceived != len(dip.buffer) {
		return nil, nil
	}
	// finished assembly of data chunks
	var buf bytes.Buffer
	for _, d := range dip.buffer {
		buf.Write(d)
	}
	delete(c.chunks, msg.msgID)
	return buf.Bytes(), nil
}
