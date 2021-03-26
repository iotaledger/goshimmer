// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// Package utxoutil contains number of utility functions useful while manipulating outputs and
// constructing transactions.
// The ConsumableOutput is a wrapper of an ledgerstate.Output which allows 'consuming' tokens and tracking
// remaining balances.
// The Builder is a flexible tansaction builder tool. It use ConsumableOutput -s to build
// arbitrary transactions along 'consume->spend' scheme.
package utxoutil
