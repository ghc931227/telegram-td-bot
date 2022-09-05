#!/bin/bash

# Run by commandline
# nohup ./telegram-td-bot -apiId 123456 -apiHash xxx -botToken xxx:xxx_xxx-x -saveDir ./ -proxyIp -proxyPort -proxyAuth -proxyPwd -onMessage true -onChannelMessage true -threadNum 3 > ./log.txt 2>&1 &

export apiId=123456

export apiHash=xxx

export botToken=xxx:xxx_xxx-x

export saveDir=./

export proxyIp=

export proxyPort=

export proxyAuth=

export proxyPwd=

export onMessage=true

export onChannelMessage=true

export threadNum=3

# Run
nohup ./telegram-td-bot > ./log.txt 2>&1 &
