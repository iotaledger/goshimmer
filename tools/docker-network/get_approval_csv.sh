#!/bin/bash

## Get the number of replicas
NUM_REPLICAS=`docker ps | grep peer_replica | wc -l`

echo "Triggering approval analysis on peer_master and $NUM_REPLICAS replicas..."

curl -s -i -H \"Accept: application/json\" -H \"Content-Type: application/json\"\
 -X GET http://localhost:8080/tools/message/approval > /dev/null &

for (( c=1; c<=NUM_REPLICAS; c++ ))
do
  docker exec --interactive peer_master /bin/bash -c\
  "curl -s -i -o /dev/null -H \"Accept: application/json\"\
  -H \"Content-Type: application/json\"\
  -X GET http://docker-network_peer_replica_$c:8080/tools/message/approval" &
done
wait
echo "Triggering approval analysis on peer_master and $NUM_REPLICAS replicas... DONE"

now=$(date +"%y%m%d_%H_%M_%S")
mkdir -p csv

echo "Copying csv files from peer_master and $NUM_REPLICAS replicas..."
docker cp peer_master:/tmp/approval-analysis.csv ./csv/$now-peer_master.csv
for (( i=1; i<=NUM_REPLICAS; i++ ))
do
  docker cp docker-network_peer_replica_$i:/tmp/approval-analysis.csv ./csv/$now-docker-network_peer_replica_$i.csv
done

echo "Copying csv files from peer_master and $NUM_REPLICAS replicas... DONE"
echo "Copied files are located at `pwd`/csv"