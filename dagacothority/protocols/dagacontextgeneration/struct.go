package dagacontextgeneration

import (
	"go.dedis.ch/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/student_18_daga/dagacothority"
	"github.com/dedis/student_18_daga/sign/daga"
)

/*
Struct holds the messages that will be sent around in the protocols. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

// Name can be used from other packages to refer to this protocols.
const Name = "DAGAContextGeneration"

// Announce is sent from Leader upon reception of a "client"(3rd-party service's admin) request.
// It request that all other nodes generate a new `daga.Server` identity
// and send back a commitment R to their new/fresh per-round secret r (R=rG)
type Announce struct {
	AssignedIndex   int // the Leader assigned index of the node's `daga.Server` under the "to be created context"
	OriginalRequest dagacothority.CreateContext
}

// StructAnnounce just contains Announce and the data necessary to identify and
// process the message in the framework.
type StructAnnounce struct {
	*onet.TreeNode // sender
	Announce
}

// AnnounceReply is sent from all other nodes back to the Leader, it contains what the leader asked,
// the public key Y of their new `daga.Server` identity and the commitment R to their fresh per-round secret r
type AnnounceReply struct {
	Y kyber.Point
	R kyber.Point
}

// StructAnnounceReply just contains AnnounceReply and the data necessary to identify and
// process the message in the framework.
type StructAnnounceReply struct {
	*onet.TreeNode
	AnnounceReply
}

// Sign is sent from Leader upon reception and processing of all AnnounceReply.
// it request approval (a signature) - from all other nodes - for the newly built context
type Sign struct {
	Context daga.MinimumAuthenticationContext // TODO DECIDE what kind of context here (include roster or not ?)
}

// StructSign just contains Sign and the data necessary to identify and
// process the message in the framework.
type StructSign struct {
	*onet.TreeNode
	Sign
}

// SignReply is sent from all nodes back to the Leader, it contains what the leader asked, their approval/signature
type SignReply struct {
	Signature []byte // schnorr signature of the context, verifiable using the context.G.Y[node index] public key
}

// StructSignReply just contains SignReply and the data necessary to identify and
// process the message in the framework.
type StructSignReply struct {
	*onet.TreeNode
	SignReply
}

// Done is sent from Leader to other nodes, contain the final result/context
// TODO consider reducing the amount of data being sent (nodes already have (or can save in previous steps) portions of context
type Done struct {
	FinalContext dagacothority.Context
}

// StructDone just contains Done and the data necessary to identify and
// process the message in the framework.
type StructDone struct {
	*onet.TreeNode
	Done
}
