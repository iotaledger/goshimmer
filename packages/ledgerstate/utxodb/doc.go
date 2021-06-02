// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// Package utxodb mocks Value Tangle ledger by implementing fully synchronous in-memory  database
// of GoShimmer value transactions. It ensures consistency of the ledger validity and all transactions
// added to the UTXODB by checking inputs, outputs and signatures.
package utxodb
