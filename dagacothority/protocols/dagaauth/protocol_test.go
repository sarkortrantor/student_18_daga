package dagaauth_test

import (
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/student_18_daga/dagacothority"
	"github.com/dedis/student_18_daga/dagacothority/protocols/dagaauth"
	protocols_testing "github.com/dedis/student_18_daga/dagacothority/testing"
	"github.com/dedis/student_18_daga/sign/daga"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

var tSuite = daga.NewSuiteEC()

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// Tests a 2, 5 and 13-node system. (complete protocol run)
func TestServerProtocol(t *testing.T) {
	nodes := []int{2, 5, 13}
	for _, nbrNodes := range nodes {
		runProtocol(t, nbrNodes)
	}
}

func runProtocol(t *testing.T, nbrNodes int) {
	log.Lvl2("Running", dagaauth.Name, "with", nbrNodes, "nodes")
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	services, dummyRequest, dummyContext := protocols_testing.ValidServiceSetup(local, nbrNodes)

	// create and setup root protocol instance + start protocol
	netRequest := dagacothority.NetEncodeAuthenticationMessage(*dummyContext, *dummyRequest)
	dagaProtocol := services[0].(*protocols_testing.DummyService).NewDAGAServerProtocol(t, *netRequest)

	serverMsg, err := dagaProtocol.WaitForResult()
	require.NoError(t, err, "failed to get result of protocol run (valid setup)")
	require.NotZero(t, serverMsg)

	// verify / extract tag
	Tf, err := daga.GetFinalLinkageTag(tSuite, dummyContext, serverMsg)
	require.NoError(t, err, "failed to extract tag from the resulting ServerMsg")
	require.NotZero(t, Tf)
}

// TODO remove the unnecessary local setup in tests that only check behavior of methods/func in isolation

func TestLeaderSetup(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	// valid setup, should not panic
	nbrNodes := 1
	_, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes-1, true)
	_, dagaServers, dummyRequest, dummyContext := protocols_testing.DummyDagaSetup(rand.Intn(10)+2, len(local.Servers), roster)
	pi, _ := local.CreateProtocol(dagaauth.Name, tree)
	defer pi.(*dagaauth.Protocol).Done()

	netRequest := dagacothority.NetEncodeAuthenticationMessage(*dummyContext, *dummyRequest)

	require.NotPanics(t, func() {
		pi.(*dagaauth.Protocol).LeaderSetup(*netRequest, dagaServers[0])
	}, "should not panic on valid input")
}

func TestLeaderSetupShouldPanicOnNilServer(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	nbrNodes := 1
	_, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes-1, true)
	_, _, dummyRequest, dummyContext := protocols_testing.DummyDagaSetup(rand.Intn(10)+2, len(local.Servers), roster)
	pi, _ := local.CreateProtocol(dagaauth.Name, tree)
	defer pi.(*dagaauth.Protocol).Done()

	netRequest := dagacothority.NetEncodeAuthenticationMessage(*dummyContext, *dummyRequest)

	require.Panics(t, func() {
		pi.(*dagaauth.Protocol).LeaderSetup(*netRequest, nil)
	}, "should panic on nil server")
}

func TestLeaderSetupShouldPanicOnInvalidState(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	nbrNodes := 1
	_, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes-1, true)
	_, dagaServers, dummyRequest, dummyContext := protocols_testing.DummyDagaSetup(rand.Intn(10)+2, len(local.Servers), roster)
	pi, _ := local.CreateProtocol(dagaauth.Name, tree)

	netRequest := dagacothority.NetEncodeAuthenticationMessage(*dummyContext, *dummyRequest)

	pi.(*dagaauth.Protocol).LeaderSetup(*netRequest, dagaServers[0])
	require.Panics(t, func() {
		pi.(*dagaauth.Protocol).LeaderSetup(*netRequest, dagaServers[0])
	}, "should panic on already initialized node")
	pi.(*dagaauth.Protocol).Done()

	pi, _ = local.CreateProtocol(dagaauth.Name, tree)
	defer pi.(*dagaauth.Protocol).Done()

	pi.(*dagaauth.Protocol).ChildSetup(func(ctx dagacothority.Context) (daga.Server, error) {
		return dagaServers[0], nil
	})
	require.Panics(t, func() {
		pi.(*dagaauth.Protocol).LeaderSetup(*netRequest, dagaServers[0])
	}, "should panic on already initialized node")
}

func TestChildrenSetup(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	// valid setup, should not panic
	nbrNodes := 1
	_, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes-1, true)
	_, dagaServers, _, _ := protocols_testing.DummyDagaSetup(rand.Intn(10)+2, len(local.Servers), roster)
	pi, _ := local.CreateProtocol(dagaauth.Name, tree)
	defer pi.(*dagaauth.Protocol).Done()

	require.NotPanics(t, func() {
		pi.(*dagaauth.Protocol).ChildSetup(func(ctx dagacothority.Context) (daga.Server, error) {
			return dagaServers[0], nil
		})
	}, "should not panic on valid input")
}

func TestChildrenSetupShouldPanicOnInvalidState(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	nbrNodes := 1
	_, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes-1, true)
	_, dagaServers, dummyRequest, dummyContext := protocols_testing.DummyDagaSetup(rand.Intn(10)+2, len(local.Servers), roster)
	pi, _ := local.CreateProtocol(dagaauth.Name, tree)

	pi.(*dagaauth.Protocol).ChildSetup(func(ctx dagacothority.Context) (daga.Server, error) {
		return dagaServers[0], nil
	})
	require.Panics(t, func() {
		pi.(*dagaauth.Protocol).ChildSetup(func(ctx dagacothority.Context) (daga.Server, error) {
			return dagaServers[0], nil
		})
	}, "should panic on already initialized node")
	pi.(*dagaauth.Protocol).Done()

	pi, _ = local.CreateProtocol(dagaauth.Name, tree)
	defer pi.(*dagaauth.Protocol).Done()

	netRequest := dagacothority.NetEncodeAuthenticationMessage(*dummyContext, *dummyRequest)

	pi.(*dagaauth.Protocol).LeaderSetup(*netRequest, dagaServers[0])
	require.Panics(t, func() {
		pi.(*dagaauth.Protocol).ChildSetup(func(ctx dagacothority.Context) (daga.Server, error) {
			return dagaServers[0], nil
		})
	}, "should panic on already initialized node")
}

func TestWaitForResultShouldPanicIfCalledBeforeStart(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	nbrNodes := 5
	_, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, 2, true)
	_, dagaServers, dummyRequest, dummyContext := protocols_testing.DummyDagaSetup(rand.Intn(10)+2, len(local.Servers), roster)
	pi, _ := local.CreateProtocol(dagaauth.Name, tree)
	defer pi.(*dagaauth.Protocol).Done()

	netRequest := dagacothority.NetEncodeAuthenticationMessage(*dummyContext, *dummyRequest)

	pi.(*dagaauth.Protocol).LeaderSetup(*netRequest, dagaServers[0])
	require.Panics(t, func() {
		pi.(*dagaauth.Protocol).WaitForResult()
	})
}

func TestWaitForResultShouldPanicOnNonRootInstance(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()

	nbrNodes := 5
	_, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, 2, true)
	_, dagaServers, _, _ := protocols_testing.DummyDagaSetup(rand.Intn(10)+2, len(local.Servers), roster)
	pi, _ := local.CreateProtocol(dagaauth.Name, tree)
	defer pi.(*dagaauth.Protocol).Done()

	// TODO test name little misleading but ..

	pi.(*dagaauth.Protocol).ChildSetup(func(ctx dagacothority.Context) (daga.Server, error) {
		return dagaServers[0], nil
	})
	require.Panics(t, func() {
		pi.(*dagaauth.Protocol).WaitForResult()
	})
}

// QUESTION TODO don't know how to test more advanced things, how to simulate bad behavior from some nodes
// now I'm only assured that it works when setup like intended + some little bad things
// but no guarantees on what happens otherwise
