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
    printf "\n == Running tests == \n"
    go test -race -v ./...
    checkExit
    printf "\n == Running benchmark (sends to results.txt) == \n"
    go test -bench=. -benchtime=100x -benchmem ./...
    checkExit
else
    echo "ERROR: unable to perform tests"
    exit 1
fi
