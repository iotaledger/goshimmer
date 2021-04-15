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
	Transactions []*TransactionEssence
}

// WriteTo writes the snapshot data to the given writer.
func (s *Snapshot) WriteTo(writer io.Writer) (int64, error) {
	var bytesWritten int64
	if err := binary.Write(writer, binary.LittleEndian, uint32(len(s.Transactions))); err != nil {
		return 0, fmt.Errorf("unable to write transactions count: %w", err)
	}
	bytesWritten += 4
	for i, transaction := range s.Transactions {
		if err := binary.Write(writer, binary.LittleEndian, uint32(len(transaction.Bytes()))); err != nil {
			return 0, fmt.Errorf("unable to write length of transaction at index %d: %w", i, err)
		}
		bytesWritten += 4

		if err := binary.Write(writer, binary.LittleEndian, transaction.Bytes()); err != nil {
			return 0, fmt.Errorf("unable to write transaction at index %d: %w", i, err)
		}
		bytesWritten += int64(len(transaction.Bytes()))
	}

	return bytesWritten, nil
}

// ReadFrom reads the snapshot bytes from the given reader.
// This function overrides existing content of the snapshot.
func (s *Snapshot) ReadFrom(reader io.Reader) (int64, error) {
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

		transactionBytes := make([]byte, transactionLength)
		if err := binary.Read(reader, binary.LittleEndian, &transactionBytes); err != nil {
			return 0, fmt.Errorf("unable to read transaction at index %d: %w", i, err)
		}

		tx, n, err := TransactionEssenceFromBytes(transactionBytes)
		if err != nil {
			return 0, fmt.Errorf("unable to parse transaction at index %d: %w", i, err)
		}
		s.Transactions = append(s.Transactions, tx)
		bytesRead += int64(n)
	}

	return bytesRead, nil
}
