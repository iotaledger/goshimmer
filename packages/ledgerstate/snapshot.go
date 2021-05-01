package ledgerstate

import (
	"encoding/binary"
	"fmt"
	"io"
)

// 1. Genesis Message (empty messageID)
// 2. Genesis Output (within genesis Transaction) -> total supply

// Make so that all the transactions contained in the snapshot are attached by the genesis message
// Store the transction info of the snapshot into the mana object storage only if the transaction was not stored already

// c-mana
// a-mana

// type snapshot []TransactionEssence
// tx1: input genesis -> output (pledge to X) 2 weeks ago

// Today
// tx2: referenced output - > 100 ouputs (pledge to empty)

// Snapshot defines a snapshot of the ledger state.
type Snapshot struct {
	Transactions map[TransactionID]Record
}

// Record defines a record of the snapshot.
type Record struct {
	Essence        *TransactionEssence
	UnpsentOutputs []bool
}

// WriteTo writes the snapshot data to the given writer.
func (s *Snapshot) WriteTo(writer io.Writer) (int64, error) {
	var bytesWritten int64
	if err := binary.Write(writer, binary.LittleEndian, uint32(len(s.Transactions))); err != nil {
		return 0, fmt.Errorf("unable to write transactions count: %w", err)
	}
	bytesWritten += 4
	for transactionID, record := range s.Transactions {
		if err := binary.Write(writer, binary.LittleEndian, uint32(len(record.Essence.Bytes()))); err != nil {
			return 0, fmt.Errorf("unable to write length of transaction with %s: %w", transactionID, err)
		}
		bytesWritten += 4

		if err := binary.Write(writer, binary.LittleEndian, transactionID.Bytes()); err != nil {
			return 0, fmt.Errorf("unable to write transactionID with %s: %w", transactionID, err)
		}
		bytesWritten += TransactionIDLength

		if err := binary.Write(writer, binary.LittleEndian, record.Essence.Bytes()); err != nil {
			return 0, fmt.Errorf("unable to write transaction with %s: %w", transactionID, err)
		}
		bytesWritten += int64(len(record.Essence.Bytes()))

		if err := binary.Write(writer, binary.LittleEndian, uint32(len(record.UnpsentOutputs))); err != nil {
			return 0, fmt.Errorf("unable to write unspent output index lengt hwith %s: %w", transactionID, err)
		}
		bytesWritten += 4

		for _, unspentOutput := range record.UnpsentOutputs {
			if err := binary.Write(writer, binary.LittleEndian, unspentOutput); err != nil {
				return 0, fmt.Errorf("unable to write unspent output index with %s: %w", transactionID, err)
			}
		}
		bytesWritten += int64(len(record.UnpsentOutputs))
	}

	return bytesWritten, nil
}

// ReadFrom reads the snapshot bytes from the given reader.
// This function overrides existing content of the snapshot.
func (s *Snapshot) ReadFrom(reader io.Reader) (int64, error) {
	s.Transactions = make(map[TransactionID]Record)
	var bytesRead int64
	var transactionCount uint32
	if err := binary.Read(reader, binary.LittleEndian, &transactionCount); err != nil {
		return 0, fmt.Errorf("unable to read transaction count: %w", err)
	}
	bytesRead += 4

	for i := 0; i < int(transactionCount); i++ {
		var transactionLength uint32
		if err := binary.Read(reader, binary.LittleEndian, &transactionLength); err != nil {
			return 0, fmt.Errorf("unable to read length of transaction at index %d: %w", i, err)
		}
		bytesRead += 4

		transactionIDBytes := make([]byte, TransactionIDLength)
		if err := binary.Read(reader, binary.LittleEndian, &transactionIDBytes); err != nil {
			return 0, fmt.Errorf("unable to read transactionID: %w", err)
		}

		txID, n, e := TransactionIDFromBytes(transactionIDBytes)
		if e != nil {
			return 0, fmt.Errorf("unable to parse transactionID at index %d: %w", i, e)
		}
		bytesRead += int64(n)

		transactionBytes := make([]byte, transactionLength)
		if err := binary.Read(reader, binary.LittleEndian, &transactionBytes); err != nil {
			return 0, fmt.Errorf("unable to read transaction at index %d: %w", i, err)
		}

		txEssence, n, err := TransactionEssenceFromBytes(transactionBytes)
		if err != nil {
			return 0, fmt.Errorf("unable to parse transaction at index %d: %w", i, err)
		}
		bytesRead += int64(n)

		var unspentOutputsLenght uint32
		if err := binary.Read(reader, binary.LittleEndian, &unspentOutputsLenght); err != nil {
			return 0, fmt.Errorf("unable to read unspent outputs length at index %d: %w", i, err)
		}
		bytesRead += 4

		unspentOutputs := make([]bool, unspentOutputsLenght)
		for j := 0; j < int(unspentOutputsLenght); j++ {
			if err := binary.Read(reader, binary.LittleEndian, &unspentOutputs[j]); err != nil {
				return 0, fmt.Errorf("unable to read unspent output at index %d: %w", j, err)
			}
		}

		bytesRead += int64(unspentOutputsLenght)

		s.Transactions[txID] = Record{
			Essence:        txEssence,
			UnpsentOutputs: unspentOutputs,
		}
	}

	return bytesRead, nil
}
