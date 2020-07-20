#!/bin/bash

if [[ $# -eq 0 ]] ; then
    echo 'Call with ./run replicas'
    exit 0
fi

REPLICAS=$1

echo "Build GoShimmer"
docker-compose -f builder/docker-compose.builder.yml up --abort-on-container-exit --exit-code-from builder

# check exit code of builder
if [ $? -ne 0 ]
then
  echo "Building failed. Please fix and try again!"
  exit 1
fi

echo "Run GoShimmer network"
docker-compose up --scale peer_replica=$REPLICAS

echo "Clean up docker network"
docker-compose down