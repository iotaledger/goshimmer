#!/bin/bash

if [ -z "$1" ]; then
    echo "Usage: `basename $0` number_of_peer_replicas"
    exit 0
fi

echo "Build GoShimmer Docker network"
docker-compose -f docker-compose.yml up -d --scale peer_replica=$1
if [ $? -ne 0 ]; then { echo "Failed, aborting." ; exit 1; } fi

echo "Dispay containers"
docker ps -a

echo "Run integration tests"
docker-compose -f tester/docker-compose.yml up --exit-code-from tester

echo "Clean up"
docker-compose -f tester/docker-compose.yml down --rmi all --remove-orphan --volumes
docker-compose -f docker-compose.yml down --rmi all --remove-orphan --volumes
