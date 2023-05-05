#!/bin/sh

while true; do
    now=$(date +"%T")
    echo "restart : $now"
    ./explorer_collect
done