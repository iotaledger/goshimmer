# Relay-Checker

This tool checks whether a transaction which is created on a given node (via `broadcastData`), 
is actually relayed/gossiped through the network by checking the transaction's existence on other 
specified nodes after a cooldown.

This program can be configured via CLI:
