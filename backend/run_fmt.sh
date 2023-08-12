#!/bin/bash
for i in $(find $(pwd) -name '*.go');do gofmt -w $i;done
