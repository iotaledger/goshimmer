package chopper

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
)

// msgChunk is a special wrapper message for chunks of larger than buffer messages.
type msgChunk struct {
	msgID       uint32
	chunkSeqNum uint16
	numChunks   uint16
	data        []byte
}

func (c *msgChunk) encode() []byte {
	m := marshalutil.New()
	m.WriteUint32(c.msgID)
	m.WriteUint16(c.numChunks)
	m.WriteUint16(c.chunkSeqNum)
	m.WriteUint16(uint16(len(c.data)))
	m.WriteBytes(c.data)
	return m.Bytes()
}

func (c *msgChunk) decode(data []byte, maxChunkSizeWithoutHeader int) error {
	m := marshalutil.New(data)
	var err error
	if c.msgID, err = m.ReadUint32(); err != nil {
		return err
	}
	if c.numChunks, err = m.ReadUint16(); err != nil {
		return err
	}
	if c.chunkSeqNum, err = m.ReadUint16(); err != nil {
		return err
	}
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	if c.data, err = m.ReadBytes(int(size)); err != nil {
		return err
	}
	if c.chunkSeqNum >= c.numChunks {
		return fmt.Errorf("wrong data chunk format")
	}
	if len(c.data) != maxChunkSizeWithoutHeader && c.chunkSeqNum != c.numChunks-1 {
		return fmt.Errorf("wrong data chunk length")
	}
	return nil
}
