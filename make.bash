#!/usr/bin/env bash

if [ ! -f make.bash ]; then
    echo 'make.bash must be run within its container folder.' 1>&2
    exit 1
fi

export GOOS=windows
go build -o tcplink.exe

export GOOS=freebsd
go build -o tcplink.bsd

export GOOS=darwin
go build -o tcplink.mac

export GOOS=linux
go build -o tcplink.lnx

