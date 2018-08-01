package client_test

import (
	"os"
	"testing"

	"github.com/ya-enot/abci/example/dummy"
	nm "github.com/ya-enot/tendermint/node"
	rpctest "github.com/ya-enot/tendermint/rpc/test"
)

var node *nm.Node

func TestMain(m *testing.M) {
	// start a tendermint node (and dummy) in the background to test against
	app := dummy.NewDummyApplication()
	node = rpctest.StartTendermint(app)
	code := m.Run()

	// and shut down proper at the end
	node.Stop()
	node.Wait()
	os.Exit(code)
}
