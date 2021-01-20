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

## How to use message approval check tool

`get_approval_csv.sh` script helps you conveniently trigger the message approval checks on all nodes in the docker
network, and gather their results in the `csv` folder.

Once the network is up and running, execute the script:
```
$ ./get_approval_csv.sh
```
Example output:
```
Triggering approval analysis on peer_master and 20 replicas...
Triggering approval analysis on peer_master and 20 replicas... DONE
Copying csv files from peer_master and 20 replicas...
Copying csv files from peer_master and 20 replicas... DONE
Copied files are located at /home/{user}/repo/goshimmer/tools/docker-network/csv
```
The exported csv files are timestamped to the date of request.
```
csv
├── 210120_16_34_14-docker-network_peer_replica_10.csv
├── 210120_16_34_14-docker-network_peer_replica_11.csv
├── 210120_16_34_14-docker-network_peer_replica_12.csv
...
```
Note, that the record length of the files might differ, since the approval check execution time of the nodes might differ.
