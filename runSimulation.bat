@echo off
SETLOCAL EnableDelayedExpansion

Set PEERING_PORT=14000

mkdir testNodes
cd testNodes

FOR /L %%i IN (1,1,100) DO (
    set /A PEERING_PORT=PEERING_PORT+1

    del /Q /S node%%i
    mkdir node%%i
    copy ..\entryNode\go_build_github_com_iotaledger_goshimmer.exe node%%i\goshimmer.exe
    cd node%%i
    start /b "" "goshimmer.exe" -autopeering-port !PEERING_PORT! -autopeering-entry-nodes e83c2e1f0ba552cbb6d08d819b7b2196332f8423@127.0.0.1:14000 -node-log-level 4 -node-disable-plugins statusscreen
    cd ..
)

cd ..