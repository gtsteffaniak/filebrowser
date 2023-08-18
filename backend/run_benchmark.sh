#!/bin/sh
## TEST file used by docker testing containers
touch render.yml
checkExit() {
    if [ "$?" -ne 0 ];then
        exit 1
    fi
}

if command -v go &> /dev/null
then
    printf "\n == Running benchmark == \n"
    go test -bench=. -benchtime=10x -benchmem ./...
    checkExit
else
    echo "ERROR: unable to perform tests"
    exit 1
fi
