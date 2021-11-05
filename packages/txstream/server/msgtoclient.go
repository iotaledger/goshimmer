package server

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream"
)

func (c *Connection) sendMsgToClient(msg txstream.Message) {
	var err error
	defer func() {
		if err != nil {
			c.log.Errorf("sending message to client (%T): %v", msg, err)
		}
	}()

	data := txstream.EncodeMsg(msg)
	choppedData, chopped, err := c.chopper.ChopData(data, tangle.MaxMessageSize, txstream.ChunkMessageHeaderSize)
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
			c.log.Panicf("sendMsgToClient: internal inconsistency: size too big: %d", len(dataToSend))
		}
		_, err = c.bconn.Write(dataToSend)
		if err != nil {
			return
		}
	}
}

func (c *Connection) sendTxInclusionState(txid ledgerstate.TransactionID, addr ledgerstate.Address, gradeOfFinality gof.GradeOfFinality) {
	c.sendMsgToClient(&txstream.MsgTxGoF{
		Address:         addr,
		TxID:            txid,
		GradeOfFinality: gradeOfFinality,
	})
}

func (c *Connection) pushTransaction(txid ledgerstate.TransactionID, addr ledgerstate.Address) {
	found := c.ledger.GetHighGoFTransaction(txid, func(tx *ledgerstate.Transaction) {
		c.sendMsgToClient(&txstream.MsgTransaction{
			Address: addr,
			Tx:      tx,
		})
	})
	if !found {
		c.log.Warnf("pushTransaction: not found %c", txid.String())
	}
}

func (c *Connection) sendOutput(outputID ledgerstate.OutputID, addr ledgerstate.Address) {
	validOutput := true
	foundTx := c.ledger.GetHighGoFTransaction(outputID.TransactionID(), func(tx *ledgerstate.Transaction) {
		idx := outputID.OutputIndex()
		if int(idx) >= len(tx.Essence().Outputs()) {
			validOutput = false
			return
		}
		validOutput = c.ledger.GetOutputMetadata(outputID, func(meta *ledgerstate.OutputMetadata) {
			c.sendMsgToClient(&txstream.MsgOutput{
				Address:        addr,
				Output:         tx.Essence().Outputs()[outputID.OutputIndex()].UpdateMintingColor(),
				OutputMetadata: meta,
			})
		})
	})
	if !foundTx || !validOutput {
		c.log.Warnf("sendOutput: not found output %s", outputID.String())
	}
}

func (c *Connection) sendUnspentAliasOutput(addr *ledgerstate.AliasAddress) {
	found := false
	c.ledger.GetUnspentOutputs(addr, func(out ledgerstate.Output) {
		if found {
			return
		}
		if aliasOut, ok := out.(*ledgerstate.AliasOutput); ok {
			var timestamp time.Time
			c.ledger.GetHighGoFTransaction(aliasOut.ID().TransactionID(), func(tx *ledgerstate.Transaction) {
				timestamp = tx.Essence().Timestamp()
			})
			c.ledger.GetOutputMetadata(out.ID(), func(meta *ledgerstate.OutputMetadata) {
				if meta.ConsumerCount() != 0 {
					return
				}
				found = true
				c.sendMsgToClient(&txstream.MsgUnspentAliasOutput{
					AliasAddress:   addr,
					AliasOutput:    aliasOut,
					OutputMetadata: meta,
					Timestamp:      timestamp,
				})
			})
		}
	})
	if !found {
		c.log.Warnf("sendUnspentAliasOutput: not found alias output for address %s", addr.Base58())
	}
}
