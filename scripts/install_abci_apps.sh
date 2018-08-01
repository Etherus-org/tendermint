#! /bin/bash

go get -d github.com/ya-enot/abci

# get the abci commit used by tendermint
COMMIT=`bash scripts/glide/parse.sh abci`

echo "Checking out vendored commit for abci: $COMMIT"

cd $GOPATH/src/github.com/ya-enot/abci
git checkout $COMMIT
glide install
go install ./cmd/...
