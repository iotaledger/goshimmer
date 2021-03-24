package server

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/txstream"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func (c *Connection) sendMsgToClient(msg txstream.Message) {
	var err error
	defer func() {
		if err != nil {
			c.log().Errorf("sendMsgToClient: %c", err.Error())
		}
	}()

	data := txstream.EncodeMsg(msg)
	choppedData, chopped, err := c.messageChopper.ChopData(data, tangle.MaxMessageSize, txstream.ChunkMessageHeaderSize)
	if err != nil {
		return
	}
	if !chopped {
		_, err = c.bconn.Write(data)
		return
	}

	// sending piece by piece wrapped in MsgChunk
	for _, piece := range choppedData {
		dataToSend := txstream.EncodeMsg(&txstream.MsgChunk{Data: piece})
		if len(dataToSend) > tangle.MaxMessageSize {
			c.log().Panicf("sendMsgToClient: internal inconsistency: size too big: %d", len(dataToSend))
		}
		_, err = c.bconn.Write(dataToSend)
		if err != nil {
			return
		}
	}
}

func (c *Connection) sendTxInclusionState(txid ledgerstate.TransactionID, addr ledgerstate.Address, state ledgerstate.InclusionState) {
	c.sendMsgToClient(&txstream.MsgTxInclusionState{
		Address: addr,
		TxID:    txid,
		State:   state,
	})
}

func (c *Connection) pushTransaction(txid ledgerstate.TransactionID, addr ledgerstate.Address) {
	found := c.ledger.GetConfirmedTransaction(txid, func(tx *ledgerstate.Transaction) {
		c.sendMsgToClient(&txstream.MsgTransaction{
			Address: addr,
			Tx:      tx,
		})
	})
	if !found {
		c.log().Warnf("pushTransaction: not found %c", txid.String())
	}
}
