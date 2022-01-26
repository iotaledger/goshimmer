# TXStream plugin

## Purpose

The TXStream plugin exposes a TCP port for clients to connect and subscribe to real-time
notifications on confirmed transactions targeted to specific addresses. It is
used by Wasp nodes to be notified about incoming requests and updates on a
given chain, but it provides a generic API, not tied to ISCP or Wasp.

## Protocol

The TXStream plugin uses a duplex, binary protocol over TCP. Messaging is
completely asynchronous: neither party is waiting for the response or
confirmation after sending a message to the other party.
Even if a message is for example a request to fetch a transaction, the client
receives the response asynchronously.
This also means that messages may be lost without notification (e.g. if the
connection drops before receiving the reply).

The list and description of messages in the protocol can be found in
`packages/txstream/msg.go`.

## Configuration

The TXStream plugin supports the following configuration value in `config.json`:

```json
"txstream": {
"bindAddress": ":5000",
}
```

- `txstream.bindAddress` specifies the TCP address for listening to new
  connections.
