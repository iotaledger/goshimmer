// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// Package utxodb mocks Value Tangle ledger by implementing fully synchronous in-memory  database
// of Goshimmer value transactions. It ensures consistency of the ledger validity and all transactions
// added to the UTXODB by checking inputs, outputs and signatures.
//
// The total supply of colored tokens in the UTXODB ledger is always equal to the number of tokens
// set in the genesis transaction: 100 * 1000 * 1000 * 1000
package utxodb
