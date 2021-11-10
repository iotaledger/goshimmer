package server

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream"

	"golang.org/x/xerrors"
)

// process first message from client.
func (c *Connection) receiveClientID(data []byte) (string, error) {
	msg, err := txstream.DecodeMsg(data, txstream.FlagClientToServer)
	if err != nil {
		return "", xerrors.Errorf("DecodeMsg: %v", err)
	}

	if msg, ok := msg.(*txstream.MsgSetID); ok {
		return msg.ClientID, nil
	}
	return "", xerrors.Errorf("wrong msg type: %T", msg)
}

// process messages received from the client.
func (c *Connection) processMessageFromClient(data []byte) error {
	msg, err := txstream.DecodeMsg(data, txstream.FlagClientToServer)
	if err != nil {
		return xerrors.Errorf("DecodeMsg: %v", err)
	}

	switch msg := msg.(type) {
	case *txstream.MsgChunk:
		finalMsg, err := c.chopper.IncomingChunk(msg.Data, tangle.MaxMessageSize, txstream.ChunkMessageHeaderSize)
		if err != nil {
			return xerrors.Errorf("IncomingChunk: %v", err)
		}
		if finalMsg != nil {
			return c.processMessageFromClient(finalMsg)
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

	case *txstream.MsgGetBacklog:
		c.getBacklog(msg.Address)

	case *txstream.MsgGetConfirmedOutput:
		c.sendOutput(msg.OutputID, msg.Address)

	case *txstream.MsgGetUnspentAliasOutput:
		c.sendUnspentAliasOutput(msg.AliasAddress)

	default:
		return xerrors.Errorf("wrong msg type: %T", msg)
	}
	return nil
}
