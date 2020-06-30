# GoShimmer Network with Docker

![Docker network](../../images/docker-network.png)

Running `./run.sh` spins up a GoShimmer network within Docker as schematically shown in the figure above.
`N` defines the number of `peer_replicas` and can be specified when running the network.
The peers can communicate freely within the Docker network 
while the analysis dashboard, `master_peer's` dashboard and web API are reachable from the host system on the respective ports.

The different containers (`entry_node`, `peer_master`, `peer_replica`) are based on the same config file 
and separate config file and modified as necessary, respectively. 

## How to use as development tool
Using a standalone throwaway Docker network can be really helpful as a development tool. 

Prerequisites: 
- Docker 17.12.0+
- Docker compose: file format 3.5

Reachable from the host system
- analysis dashboard (autopeering visualizer): http://localhost:9000
- `master_peer's` dashboard: http: http://localhost:8081
- `master_peer's` web API: http: http://localhost:8080

It is therefore possible to send messages to the local network via the `master_peer` and observe log messages either 
via `docker logs --follow CONTAINER` or all of them combined when running via:

```
./run.sh 5
```