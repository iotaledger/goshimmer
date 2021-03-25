package server

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream"
)

// process messages received from the clien
func (c *Connection) processMessageFromClient(data []byte) {
	var msg interface{}
	var err error
	if msg, err = txstream.DecodeMsg(data, txstream.FlagClientToServer); err != nil {
		c.log().Errorf("DecodeMsg: %v", err)
		return
	}
	switch msg := msg.(type) {
	case *txstream.MsgChunk:
		finalMsg, err := c.messageChopper.IncomingChunk(msg.Data, tangle.MaxMessageSize, txstream.ChunkMessageHeaderSize)
		if err != nil {
			c.log().Errorf("DecodeMsg: %v", err)
			return
		}
		if finalMsg != nil {
			c.processMessageFromClient(finalMsg)
		}

	case *txstream.MsgPostTransaction:
		c.postTransaction(msg.Tx)

	case *txstream.MsgUpdateSubscriptions:
		newAddrs := c.setSubscriptions(msg.Addresses)
		// send backlogs of newly subscribed addresses
		for _, addr := range newAddrs {
			c.getBacklog(addr)
		}

	case *txstream.MsgGetConfirmedTransaction:
		c.pushTransaction(msg.TxID, msg.Address)

	case *txstream.MsgGetTxInclusionState:
		c.getTxInclusionState(msg.TxID, msg.Address)

	case *txstream.MsgGetBacklog:
		c.getBacklog(msg.Address)

	case *txstream.MsgSetID:
		c.setID(msg.ClientID)

	default:
		panic("wrong msg type")
	}
}
