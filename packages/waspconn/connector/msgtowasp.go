package connector

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/waspconn"
)

func (wconn *WaspConnector) sendMsgToWasp(msg waspconn.Message) {
	var err error
	defer func() {
		if err != nil {
			wconn.log().Errorf("sendMsgToWasp: %s", err.Error())
		}
	}()

	data := waspconn.EncodeMsg(msg)
	choppedData, chopped, err := wconn.messageChopper.ChopData(data, tangle.MaxMessageSize, waspconn.ChunkMessageHeaderSize)
	if err != nil {
		return
	}
	if !chopped {
		_, err = wconn.bconn.Write(data)
		return
	}

	// sending piece by piece wrapped in WaspMsgChunk
	for _, piece := range choppedData {
		dataToSend := waspconn.EncodeMsg(&waspconn.WaspMsgChunk{
			Data: piece,
		})
		if len(dataToSend) > tangle.MaxMessageSize {
			wconn.log().Panicf("sendMsgToWasp: internal inconsistency: size too big: %d", len(dataToSend))
		}
		_, err = wconn.bconn.Write(dataToSend)
		if err != nil {
			return
		}
	}
}

func (wconn *WaspConnector) sendTxInclusionStateToWasp(txid ledgerstate.TransactionID, chainAddress *ledgerstate.AliasAddress, state ledgerstate.InclusionState) {
	wconn.sendMsgToWasp(&waspconn.WaspFromNodeTxInclusionStateMsg{
		ChainAddress: chainAddress,
		TxID:         txid,
		State:        state,
	})
}

func (wconn *WaspConnector) pushTransaction(txid ledgerstate.TransactionID, chainAddress *ledgerstate.AliasAddress) {
	found := wconn.ledger.GetConfirmedTransaction(txid, func(tx *ledgerstate.Transaction) {
		wconn.sendMsgToWasp(&waspconn.WaspFromNodeTransactionMsg{
			ChainAddress: chainAddress,
			Tx:           tx,
		})
	})
	if !found {
		wconn.log().Warnf("pushTransaction: not found %s", txid.String())
	}
}
