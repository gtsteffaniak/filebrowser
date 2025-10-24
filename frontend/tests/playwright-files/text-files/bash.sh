#!/bin/bash

# Simple bash script to showcase theming
counter=1
while [ $counter -le 5 ]; do
    if [ $counter -eq 3 ]; then
        echo "Counter is exactly 3!"
    elif [ $counter -gt 3 ]; then
        echo "Counter is greater than 3: $counter"
    else
        echo "Counter is less than 3: $counter"
    fi
    counter=$((counter + 1))
done
