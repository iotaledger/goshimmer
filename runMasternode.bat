@echo off
SETLOCAL EnableDelayedExpansion

cd cprng
go build -o prng.exe
start /b "" "prng.exe" -interval=2 -port=10000
cd ..

Set PEERING_PORT=14000

mkdir entryNode
cd entryNode

copy ..\goshimmer.exe goshimmer.exe
start /b "" "goshimmer.exe" -autopeering-port %PEERING_PORT% -autopeering-entry-nodes e83c2e1f0ba552cbb6d08d819b7b2196332f8423@127.0.0.1:14000 -node-log-level 4 -node-disable-plugins statusscreen

cd ..