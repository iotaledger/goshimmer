#!/bin/bash

if [ -z "$1" ]; then
    echo "Usage: `basename $0` number_of_nodes"
    exit 0
fi

re='^[0-9]+$'
if ! [[ $1 =~ $re ]] ; then
   echo "Error: Number of nodes given is not a number" >&2; exit 1
fi

PEERING_PORT=14000

if [ -d testNodes ]; then
    rm -r testNodes
fi
mkdir testNodes
cd testNodes

for i in `seq 1 $1`; do
    PEERING_PORT=$((PEERING_PORT+1))
    mkdir node_$i
    cp ../entryNode/shimmer node_$i/
    cd node_$i
    ./shimmer -autopeering-port $PEERING_PORT -autopeering-entry-nodes e83c2e1f0ba552cbb6d08d819b7b2196332f8423@127.0.0.1:14000 -node-log-level 4 -node-disable-plugins statusscreen
    cd ..
done