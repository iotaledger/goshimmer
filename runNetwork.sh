  
#!/bin/bash

if [ -z "$1" ]; then
    echo "Usage: `basename $0` number_of_nodes"
    exit 0
fi

re='^[0-9]+$'
if ! [[ $1 =~ $re ]] ; then
   echo "Error: Number of nodes given is not a number" >&2; exit 1
fi

PEERING_PORT=14630
GOSSIP_PORT=15670

if [ -d testNodes ]; then
    rm -r testNodes
fi
mkdir testNodes
cd testNodes

for i in `seq 1 $1`; do
    PEERING_PORT=$((PEERING_PORT+1))
    GOSSIP_PORT=$((GOSSIP_PORT+1))
    mkdir node_$i
    mkdir node_$i/logs
    cp ../goshimmer node_$i/
    cd node_$i
    ./goshimmer --autopeering.port $PEERING_PORT --gossip.port $GOSSIP_PORT --autopeering.address 127.0.0.1 --autopeering.entryNodes 2TwlC5mtYVrCHNKG8zkFWmEUlL0pJPS1DOOC2U4yjwo=@127.0.0.1:14626 --node.LogLevel 4 --node.disablePlugins statusscreen --analysis.serverAddress 127.0.0.1:188 &
    cd ..
done