@echo off

@REM Run by commandline
@REM telegram-td-bot.exe -apiId 123456 -apiHash xxx -botToken xxx:xxx_xxx-x -saveDir ./ -proxyIp -proxyPort -proxyAuth -proxyPwd -onMessage true -onChannelMessage true -threadNum 3

set apiId=123456

set apiHash=xxx

set botToken=xxx:xxx_xxx-x

set saveDir=./

set proxyIp=

set proxyPort=

set proxyAuth=

set proxyPwd=

set onMessage=true

set onChannelMessage=true

set threadNum=3

@REM Run
telegram-td-bot.exe

