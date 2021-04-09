package server

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream"
)

func (c *Connection) sendMsgToClient(msg txstream.Message) {
	var err error
	defer func() {
		if err != nil {
			c.log().Errorf("sendMsgToClient: %s", err.Error())
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

func (c *Connection) sendOutput(outputID ledgerstate.OutputID, addr ledgerstate.Address) {
	validOutput := true
	foundTx := c.ledger.GetConfirmedTransaction(outputID.TransactionID(), func(tx *ledgerstate.Transaction) {
		idx := outputID.OutputIndex()
		if int(idx) >= len(tx.Essence().Outputs()) {
			validOutput = false
			return
		}
		c.sendMsgToClient(&txstream.MsgOutput{
			Address: addr,
			Output:  tx.Essence().Outputs()[outputID.OutputIndex()].UpdateMintingColor(),
		})
	})
	if !foundTx || !validOutput {
		c.log().Warnf("sendOutput: not found output %s", outputID.String())
	}
}
