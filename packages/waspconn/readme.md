# WaspConn plugin for Goshimmer

## Purpose

The _WaspConn_ plugin handles connection with Wasp nodes, it is a Wasp proxy
in Goshimmer.

One or several Wasp nodes can be connected to one Goshimmer node. The Wasp node
can only be connected to one Goshimmer node (this may change in the future).

Utxodb is used for testing purposes. Implemented 
`utxodb` API endpoints are also used just for testing purposes.   

## Dependency

WaspConn is not dependent on Wasp. Instead, Wasp requires a Goshimmer node with the WaspConn
plugin.

WaspConn and Goshimmer are unaware about smart contract transactions. They treat it as just
ordinary value transaction with data payloads. 

## Protocol

WaspConn implements its part of the protocol between Wasp node and Goshimmer. 

- The protocol is completely **asynchronous messaging**: neither party is waiting for the response or
confirmation after sending a message to the other party.
Even if a message is a request for example to get a transaction, the Wasp node receives
the response asynchronously.
This also means that messages may be lost without notification.

The transport between Goshimmer and Wasp uses `BufferedConnection` provided by `hive.go`.
The protocol can handle practically unlimited message sizes.

### Messages

The list and description of messages in the protocol can be found in `msg.go`.

## Configuration

All configuration values for the WaspConn plugin are in the `waspconn` portion of the `config.json` file.

```
  "waspconn": {
    "port": 5000,
    "utxodbenabled": true,
  }
```

- `waspconn.port` specifies port where WaspCon is listening for new Wasp connections.
- `waspconn.utxodbenabled` is a boolean flag which specifies if WaspConn is mocking the value tangle (`true`) or
accessing the tangle provided by Goshimmer.
