#!/bin/bash

if [[ $# -eq 0 ]] ; then
    echo 'Call with ./run replicas'
    exit 0
fi

REPLICAS=$1

echo "Build GoShimmer"
docker-compose -f builder/docker-compose.builder.yml up

echo "Run GoShimmer network"
COMPOSE_PARALLEL_LIMIT=10 docker-compose up --scale peer_replica=$REPLICAS

echo "Clean up docker network"
docker-compose down