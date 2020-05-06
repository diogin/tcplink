#!/bin/env bash

if [ ! -f make.bash ]; then
    echo 'make.bash must be run within its container folder.' 1>&2
    exit 1
fi

go build
