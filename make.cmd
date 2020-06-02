@echo off

cd %~dp0

echo building, please be patient...

set GOOS=linux
go build -o tcplink.lnx

set GOOS=freebsd
go build -o tcplink.bsd

set GOOS=darwin
go build -o tcplink.mac

set GOOS=windows
go build -o tcplink.exe
