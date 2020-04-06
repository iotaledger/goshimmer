#!/bin/bash

if [ -z "$1" ]; then
    echo "Usage: `basename $0` number_of_peer_replicas"
    exit 0
fi

REPLICAS=$1

echo "Build GoShimmer Docker network"
docker-compose -f docker-compose.yml up -d --scale peer_replica=$REPLICAS
if [ $? -ne 0 ]; then { echo "Failed, aborting." ; exit 1; } fi

echo "Dispay containers"
docker ps -a

echo "Run integration tests"
docker-compose -f tester/docker-compose.yml up --exit-code-from tester

echo "Create logs from containers in network"
docker-compose -f docker-compose.yml stop
docker logs tester > logs/tester.log
docker logs entry_node > logs/entry_node.log
docker logs peer_master > logs/peer_master.log
for (( c=1; c<=$REPLICAS; c++ ))
do
docker logs integration-tests_peer_replica_$c > logs/peer_replica_$c.log
done

echo "Clean up"
docker-compose -f tester/docker-compose.yml down --rmi all --remove-orphan --volumes
docker-compose -f docker-compose.yml down --rmi all --remove-orphan --volumes
