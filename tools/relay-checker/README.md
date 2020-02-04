# Relay-Checker

This tool checks whether a transaction which is created on a given node (via `broadcastData`), 
is actually relayed/gossiped through the network by checking the transaction's existence on other 
specified nodes after a cooldown.

This program can be configured via CLI flags:
```
--relayChecker.cooldownTime int    the cooldown time after broadcasting the data on the specified target node (default 10)
--relayChecker.data string         data to broadcast (default "TEST99BROADCAST99DATA")
--relayChecker.repeat int          the amount of times to repeat the relay-checker queries (default 1)
--relayChecker.targetNode string   the target node from the which transaction will be broadcasted from (default "http://127.0.0.1:8080")
--relayChecker.testNodes strings   the list of nodes to check after the cooldown
--relayChecker.txAddress string    the transaction address (default "SHIMMER99TEST99999999999999999999999999999999999999999999999999999999999999999999")
```