package txstream

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// MessageType represents the type of a message in the txstream protocol.
type MessageType byte

const (
	// FlagClientToServer is set in a message type if the message is client to server.
	FlagClientToServer = byte(0x80)
	// FlagServerToClient is set in a message type if the message is server to client.
	FlagServerToClient = byte(0x40)

	msgTypeChunk = MessageType((FlagClientToServer | FlagServerToClient) + iota)

	msgTypePostTransaction = MessageType(FlagClientToServer + iota)
	msgTypeSubscribe
	msgTypeGetConfirmedTransaction
	msgTypeGetConfirmedOutput
	msgTypeGetUnspentAliasOutput
	msgTypeGetTxInclusionState
	msgTypeGetBacklog
	msgTypeSetID

	msgTypeTransaction = MessageType(FlagServerToClient + iota)
	msgTypeTxGoF
	msgTypeOutput
	msgTypeUnspentAliasOutput
)

// Message is the common interface of all messages in the txstream protocol.
type Message interface {
	Write(w *marshalutil.MarshalUtil)
	Read(r *marshalutil.MarshalUtil) error
	Type() MessageType
}

// ChunkMessageHeaderSize is the amount of bytes added by MsgChunk as overhead to each chunk.
const ChunkMessageHeaderSize = 3

// MsgChunk is a special message for big data packets chopped into pieces.
type MsgChunk struct {
	Data []byte
}

// region client --> server

// MsgPostTransaction is a request from the client to post a
// transaction in the ledger.
// No reply from server.
type MsgPostTransaction struct {
	Tx *ledgerstate.Transaction
}

// MsgUpdateSubscriptions is a request from the client to subscribe to
// requests/transactions for the given addresses. Server will send
// all transactions containing unspent outputs to the address,
// and then whenever a relevant transaction is confirmed
// in the ledger, i will be sent in real-time.
type MsgUpdateSubscriptions struct {
	Addresses []ledgerstate.Address
}

// MsgGetConfirmedTransaction is a request to get a specific confirmed
// transaction from the ledger. Server replies with MsgTransaction.
type MsgGetConfirmedTransaction struct {
	Address ledgerstate.Address
	TxID    ledgerstate.TransactionID
}

// MsgGetConfirmedOutput is a request to get a specific confirmed output from the ledger.
// It may or may not be consumed.
type MsgGetConfirmedOutput struct {
	Address  ledgerstate.Address
	OutputID ledgerstate.OutputID
}

// MsgGetUnspentAliasOutput is a request to get the unique unspent AliasOutput for the given AliasAddress.
type MsgGetUnspentAliasOutput struct {
	AliasAddress *ledgerstate.AliasAddress
}

// MsgGetTxInclusionState is a request to get the inclusion state for a transaction.
// Server replies with MsgTxInclusionState.
type MsgGetTxInclusionState struct {
	Address ledgerstate.Address
	TxID    ledgerstate.TransactionID
}

// MsgGetBacklog is a request to get the backlog for the given address. Server replies
// sending one MsgTransaction for each transaction with unspent outputs targeted
// to the address.
type MsgGetBacklog struct {
	Address ledgerstate.Address
}

// MsgSetID is a message from client informing its ID, used mostly for tracing/loging.
type MsgSetID struct {
	ClientID string
}

// endregion

// region server --> client

// MsgTransaction informs the client of a given confirmed transaction in the ledger.
type MsgTransaction struct {
	// Address is the address that requested the transaction
	Address ledgerstate.Address
	// Tx is the transaction being sent
	Tx *ledgerstate.Transaction
}

// MsgTxGoF informs the client with the GoF of a given
// transaction as a response from the given address.
type MsgTxGoF struct {
	Address         ledgerstate.Address
	TxID            ledgerstate.TransactionID
	GradeOfFinality gof.GradeOfFinality
}

// MsgOutput is the response for MsgGetConfirmedOutput.
type MsgOutput struct {
	Address        ledgerstate.Address
	Output         ledgerstate.Output
	OutputMetadata *ledgerstate.OutputMetadata
}

// MsgUnspentAliasOutput is the response for MsgGetUnspentAliasOutput.
type MsgUnspentAliasOutput struct {
	AliasAddress   *ledgerstate.AliasAddress
	AliasOutput    *ledgerstate.AliasOutput
	OutputMetadata *ledgerstate.OutputMetadata
	Timestamp      time.Time
}

// endregion

// EncodeMsg encodes the given Message as a byte slice.
func EncodeMsg(msg Message) []byte {
	m := marshalutil.New()
	m.WriteByte(byte(msg.Type()))
	msg.Write(m)
	return m.Bytes()
}

// DecodeMsg decodes a Message from the given bytes.
func DecodeMsg(data []byte, expectedFlags uint8) (interface{}, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("wrong message")
	}
	var ret Message

	msgType := MessageType(data[0])
	if uint8(msgType)&expectedFlags == 0 {
		return nil, fmt.Errorf("unexpected message")
	}

	switch msgType {
	case msgTypeChunk:
		ret = new(MsgChunk)

	case msgTypePostTransaction:
		ret = new(MsgPostTransaction)

	case msgTypeSubscribe:
		ret = new(MsgUpdateSubscriptions)

	case msgTypeGetConfirmedTransaction:
		ret = new(MsgGetConfirmedTransaction)

	case msgTypeGetTxInclusionState:
		ret = new(MsgGetTxInclusionState)

	case msgTypeGetBacklog:
		ret = new(MsgGetBacklog)

	case msgTypeSetID:
		ret = new(MsgSetID)

	case msgTypeTransaction:
		ret = new(MsgTransaction)

	case msgTypeTxGoF:
		ret = new(MsgTxGoF)

	case msgTypeGetConfirmedOutput:
		ret = new(MsgGetConfirmedOutput)

	case msgTypeGetUnspentAliasOutput:
		ret = new(MsgGetUnspentAliasOutput)

	case msgTypeOutput:
		ret = new(MsgOutput)

	case msgTypeUnspentAliasOutput:
		ret = new(MsgUnspentAliasOutput)

	default:
		return nil, fmt.Errorf("unknown message type %d", msgType)
	}
	if err := ret.Read(marshalutil.New(data[1:])); err != nil {
		return nil, err
	}
	return ret, nil
}

func (msg *MsgPostTransaction) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Tx)
}

func (msg *MsgPostTransaction) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Tx, err = new(ledgerstate.Transaction).FromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

// Type returns the Message type.
func (msg *MsgPostTransaction) Type() MessageType {
	return msgTypePostTransaction
}

func (msg *MsgUpdateSubscriptions) Write(w *marshalutil.MarshalUtil) {
	w.WriteUint16(uint16(len(msg.Addresses)))
	for _, addr := range msg.Addresses {
		w.Write(addr)
	}
}

func (msg *MsgUpdateSubscriptions) Read(m *marshalutil.MarshalUtil) error {
	var err error
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	msg.Addresses = make([]ledgerstate.Address, size)
	for i := uint16(0); i < size; i++ {
		if msg.Addresses[i], err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
			return err
		}
	}
	return nil
}

// Type returns the Message type.
func (msg *MsgUpdateSubscriptions) Type() MessageType {
	return msgTypeSubscribe
}

func (msg *MsgGetConfirmedTransaction) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
	w.Write(msg.TxID)
}

func (msg *MsgGetConfirmedTransaction) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	msg.TxID, err = ledgerstate.TransactionIDFromMarshalUtil(m)
	return err
}

// Type returns the Message type.
func (msg *MsgGetConfirmedTransaction) Type() MessageType {
	return msgTypeGetConfirmedTransaction
}

func (msg *MsgGetTxInclusionState) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
	w.Write(msg.TxID)
}

func (msg *MsgGetTxInclusionState) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.TxID, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

// Type returns the Message type.
func (msg *MsgGetTxInclusionState) Type() MessageType {
	return msgTypeGetTxInclusionState
}

func (msg *MsgGetBacklog) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
}

func (msg *MsgGetBacklog) Read(m *marshalutil.MarshalUtil) error {
	var err error
	msg.Address, err = ledgerstate.AddressFromMarshalUtil(m)
	return err
}

// Type returns the Message type.
func (msg *MsgGetBacklog) Type() MessageType {
	return msgTypeGetBacklog
}

func (msg *MsgSetID) Write(w *marshalutil.MarshalUtil) {
	w.WriteUint16(uint16(len(msg.ClientID)))
	w.WriteBytes([]byte(msg.ClientID))
}

func (msg *MsgSetID) Read(m *marshalutil.MarshalUtil) error {
	var err error
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	var clientID []byte
	if clientID, err = m.ReadBytes(int(size)); err != nil {
		return err
	}
	msg.ClientID = string(clientID)
	return nil
}

// Type returns the Message type.
func (msg *MsgSetID) Type() MessageType {
	return msgTypeSetID
}

func (msg *MsgTransaction) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
	w.Write(msg.Tx)
}

func (msg *MsgTransaction) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.Tx, err = new(ledgerstate.Transaction).FromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

// Type returns the Message type.
func (msg *MsgTransaction) Type() MessageType {
	return msgTypeTransaction
}

func (msg *MsgTxGoF) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
	w.Write(msg.GradeOfFinality)
	w.Write(msg.TxID)
}

func (msg *MsgTxGoF) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.TxID, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

// Type returns the Message type.
func (msg *MsgTxGoF) Type() MessageType {
	return msgTypeTxGoF
}

func (msg *MsgGetConfirmedOutput) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
	w.Write(msg.OutputID)
}

func (msg *MsgGetConfirmedOutput) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	msg.OutputID, err = ledgerstate.OutputIDFromMarshalUtil(m)
	return err
}

// Type returns the Message type.
func (msg *MsgGetConfirmedOutput) Type() MessageType {
	return msgTypeGetConfirmedOutput
}

func (msg *MsgGetUnspentAliasOutput) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.AliasAddress)
}

func (msg *MsgGetUnspentAliasOutput) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.AliasAddress, err = ledgerstate.AliasAddressFromMarshalUtil(m); err != nil {
		return err
	}
	return err
}

// Type returns the Message type.
func (msg *MsgGetUnspentAliasOutput) Type() MessageType {
	return msgTypeGetUnspentAliasOutput
}

func (msg *MsgOutput) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
	w.Write(msg.Output)
	w.Write(msg.Output.ID()) // we need it because output ID is not persisted in the output itself
	w.Write(msg.OutputMetadata)
}

func (msg *MsgOutput) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.Output, err = ledgerstate.OutputFromMarshalUtil(m); err != nil {
		return err
	}
	id, err := ledgerstate.OutputIDFromMarshalUtil(m)
	if err != nil {
		return err
	}
	msg.Output.SetID(id)
	if msg.OutputMetadata, err = new(ledgerstate.OutputMetadata).FromMarshalUtil(m); err != nil {
		return err
	}
	return err
}

// Type returns the Message type.
func (msg *MsgOutput) Type() MessageType {
	return msgTypeOutput
}

func (msg *MsgUnspentAliasOutput) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.AliasAddress)
	w.Write(msg.AliasOutput)
	w.Write(msg.AliasOutput.ID()) // we need it because output ID is not persisted in the output itself
	w.WriteTime(msg.Timestamp)
	w.Write(msg.OutputMetadata)
}

func (msg *MsgUnspentAliasOutput) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.AliasAddress, err = ledgerstate.AliasAddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.AliasOutput, err = new(ledgerstate.AliasOutput).FromMarshalUtil(m); err != nil {
		return err
	}
	id, err := ledgerstate.OutputIDFromMarshalUtil(m)
	if err != nil {
		return err
	}
	msg.AliasOutput.SetID(id)
	if msg.Timestamp, err = m.ReadTime(); err != nil {
		return err
	}
	if msg.OutputMetadata, err = new(ledgerstate.OutputMetadata).FromMarshalUtil(m); err != nil {
		return err
	}
	return err
}

// Type returns the Message type.
func (msg *MsgUnspentAliasOutput) Type() MessageType {
	return msgTypeUnspentAliasOutput
}

func (msg *MsgChunk) Write(w *marshalutil.MarshalUtil) {
	w.WriteUint16(uint16(len(msg.Data)))
	w.WriteBytes(msg.Data)
}

func (msg *MsgChunk) Read(m *marshalutil.MarshalUtil) error {
	var err error
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	msg.Data, err = m.ReadBytes(int(size))
	return err
}

// Type returns the Message type.
func (msg *MsgChunk) Type() MessageType {
	return msgTypeChunk
}
