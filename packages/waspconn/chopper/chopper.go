// Package chopper helps splitting messages into smaller pieces and reassemble them
package chopper

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

const (
	// for the final data packet to be not bigger than tangle.MaxMessageSize
	// 4 - chunk id, 1 seq nr, 1 num chunks, 2 - data len
	chunkHeaderSize = 4 + 1 + 1 + 2
	maxTTL          = 5 * time.Minute
)

type Chopper struct {
	nextID  uint32
	mutex   *sync.Mutex
	chunks  map[uint32]*dataInProgress
	closeCh chan bool
}

type dataInProgress struct {
	buffer      [][]byte
	ttl         time.Time
	numReceived int
}

func NewChopper() *Chopper {
	c := Chopper{
		nextID:  0,
		mutex:   &sync.Mutex{},
		chunks:  make(map[uint32]*dataInProgress),
		closeCh: make(chan bool),
	}
	go c.cleanupLoop()
	return &c
}

func (c *Chopper) Close() {
	close(c.closeCh)
}

func (c *Chopper) cleanupLoop() {
	for {
		select {
		case <-c.closeCh:
			return
		case <-time.After(10 * time.Second):
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

func (c *Chopper) getNextMsgId() uint32 {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.nextID++
	return c.nextID
}

func NumChunks(dataLen, maxMsgSize, includingChoppingOverhead int) (byte, int, error) {
	if dataLen <= maxMsgSize {
		return 0, 0, nil // no need to split
	}
	maxChunkSize := maxMsgSize - includingChoppingOverhead
	maxSizeWithoutHeader := maxChunkSize - chunkHeaderSize
	if dataLen > maxChunkSize*255 {
		return 0, 0, fmt.Errorf("chopper.NumChunks: too long data to chop")
	}
	numChunks := dataLen / maxSizeWithoutHeader
	if dataLen%maxSizeWithoutHeader > 0 {
		numChunks++
	}
	if numChunks < 2 {
		panic("ChopData: internal inconsistency 1")
	}
	return byte(numChunks), maxChunkSize, nil
}

// ChopData chops data into pieces (not more than 255) and adds chopper header to each piece
// for IncomingChunk function to reassemble it
func (c *Chopper) ChopData(data []byte, maxMsgSize int, includingChoppingOverhead int) ([][]byte, bool, error) {
	numChunks, maxChunkSize, err := NumChunks(len(data), maxMsgSize, includingChoppingOverhead)
	if err != nil {
		return nil, false, err
	}
	if numChunks == 0 {
		return nil, false, nil
	}
	maxSizeWithoutHeader := maxChunkSize - chunkHeaderSize
	id := c.getNextMsgId()
	ret := make([][]byte, 0, numChunks)
	var d []byte
	for i := byte(0); i < numChunks; i++ {
		if len(data) > maxSizeWithoutHeader {
			d = data[:maxSizeWithoutHeader]
			data = data[maxSizeWithoutHeader:]
		} else {
			d = data
		}
		chunk := &msgChunk{
			msgId:       id,
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
// maxChunkSize parameter must be the same on both sides
func (c *Chopper) IncomingChunk(data []byte, maxMsgSize int, includingChoppingOverhead int) ([]byte, error) {
	maxChunkSize := maxMsgSize - includingChoppingOverhead
	maxSizeWithoutHeader := maxChunkSize - chunkHeaderSize
	msg := msgChunk{}
	if err := msg.decode(data, maxSizeWithoutHeader); err != nil {
		return nil, err
	}
	switch {
	case len(msg.data) > maxChunkSize:
		return nil, fmt.Errorf("IncomingChunk: too long data chunk")

	case msg.chunkSeqNum >= msg.numChunks:
		return nil, fmt.Errorf("IncomingChunk: wrong incoming data chunk seq number")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	dip, ok := c.chunks[msg.msgId]
	if !ok {
		dip = &dataInProgress{
			buffer: make([][]byte, int(msg.numChunks)),
			ttl:    time.Now().Add(maxTTL),
		}
		c.chunks[msg.msgId] = dip
	} else {
		if dip.buffer[msg.chunkSeqNum] != nil {
			return nil, fmt.Errorf("IncomingChunk: repeating seq number")
		}
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
	delete(c.chunks, msg.msgId)
	return buf.Bytes(), nil
}
