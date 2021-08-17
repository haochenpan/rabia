#!/bin/bash

# throw this file in go/src before running
branches=("paxos-batching-data-size-256B"
"epaxos-batching-data-size-256B"
"paxos-batching"
"epaxos-batching"
"paxos-no-batching"
"epaxos-no-batching"
)



export GO111MODULE=auto # maybe mannualy run?
export GOPATH=${GOPATH}:~/go/src/epaxos  #  maybe mannualy run?
cd epaxos
for branch in ${branches[@]}; do
    echo $branch
    git checkout origin/$branch
    . compile.sh
    mkdir -p ../bin/$branch
    mv bin/* ../bin/$branch
    rm -rf bin
done
cd ..