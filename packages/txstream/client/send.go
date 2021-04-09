package client

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/txstream"
)

func (n *Client) sendWaspID() {
	n.sendMessage(&txstream.MsgSetID{ClientID: n.clientID})
}

// RequestBacklog requests the backlog for a given address
func (n *Client) RequestBacklog(addr ledgerstate.Address) {
	n.sendMessage(&txstream.MsgGetBacklog{Address: addr})
}

// RequestConfirmedTransaction requests a specific confirmed transaction
func (n *Client) RequestConfirmedTransaction(addr ledgerstate.Address, txid ledgerstate.TransactionID) {
	n.sendMessage(&txstream.MsgGetConfirmedTransaction{
		Address: addr,
		TxID:    txid,
	})
}

// RequestTxInclusionState requests the inclusion state of a transaction
func (n *Client) RequestTxInclusionState(addr ledgerstate.Address, txid ledgerstate.TransactionID) {
	n.sendMessage(&txstream.MsgGetTxInclusionState{
		Address: addr,
		TxID:    txid,
	})
}

// RequestTxInclusionState requests the inclusion state of a transaction
func (n *Client) RequestConfirmedOutput(addr ledgerstate.Address, outputID ledgerstate.OutputID) {
	n.sendMessage(&txstream.MsgGetConfirmedOutput{
		Address:  addr,
		OutputID: outputID,
	})
}

// PostTransaction posts a transaction to the ledger
func (n *Client) PostTransaction(tx *ledgerstate.Transaction, fromSc ledgerstate.Address, fromLeader uint16) {
	n.sendMessage(&txstream.MsgPostTransaction{Tx: tx})
}
