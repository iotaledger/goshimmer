@echo off
SETLOCAL EnableDelayedExpansion

Set PEERING_PORT=14000

cd entryNode
start /b "" "goshimmer.exe" -autopeering-port %PEERING_PORT% -autopeering-entry-nodes e83c2e1f0ba552cbb6d08d819b7b2196332f8423@127.0.0.1:14000 -node-log-level 4 -node-disable-plugins statusscreen
cd ..