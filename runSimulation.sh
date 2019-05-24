#!/bin/bash

PEERING_PORT=14000
PEERING_PORT=$((PEERING_PORT+1))
echo $PEERING_PORT 

mkdir testNodes
cd testNodes

for i in `seq 1 5`; do
    mkdir node_$i
    cp ../entryNode/shimmer node_$i 
done