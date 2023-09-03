#!/bin/sh
## TEST file used by docker testing containers
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
else
    echo "ERROR: unable to perform tests"
    exit 1
fi
