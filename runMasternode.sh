#!/bin/bash

if [ ! -d entryNode ]; then
    mkdir entryNode
fi
echo "Building shimmer..."
go build -o entryNode/shimmer
PORT=14000
cd entryNode
echo "Starting master node"
./shimmer -autopeering-port $PORT -autopeering-entry-nodes e83c2e1f0ba552cbb6d08d819b7b2196332f8423@127.0.0.1:$PORT -node-log-level 4 -node-disable-plugins statusscreen