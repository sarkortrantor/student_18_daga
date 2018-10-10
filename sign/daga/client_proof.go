package daga

// TODO organize those in a "sub package" of daga "clientProof" and rename everything with better names

import (
	"errors"
	"fmt"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/proof"
	"strconv"
)

/*Challenge stores the collectively generated challenge and the signatures of the servers
This is the structure sent to the client*/
type Challenge struct {
	cs   kyber.Scalar
	sigs []serverSignature
}

func (c Challenge) verifySignatures(serverKeys []kyber.Point) error {
	//	verify challenge signatures
	if msg, e := c.cs.MarshalBinary(); e == nil {
		for _, sig := range c.sigs {
			if e = ECDSAVerify(serverKeys[sig.index], msg, sig.sig); e != nil {
				return errors.New("failed to verify signature of server " + strconv.Itoa(sig.index) + ": " + e.Error())
			}
		}
		return nil
	} else {
		return errors.New("Error while marshalling challenge: " + e.Error())
	}
}

// Sigma-protocol proof.VerifierContext used to conduct interactive proofs with a prover
type clientVerifierCtx struct {
	SuiteProof
	commitsChan       chan kyber.Point    // to give prover's commitments to the Verifier
	challengeChan     chan kyber.Scalar   // to give master challenge to the Verifier (via PubRand)
	subChallengesChan chan []kyber.Scalar // to give the Prover generated sub-challenges to the Verifier (via Get)
	responsesChan     chan kyber.Scalar   // to give prover's responses to the Verifier (via Get)
}

// TODO doc
// n = #clients in auth. group = #predicates in OrProof
func newClientVerifierCtx(suite Suite, n int) *clientVerifierCtx {
	// FIXME: see if/where I need to deep copy passed DATA !!
	return &clientVerifierCtx{
		SuiteProof:        newSuiteProof(suite),
		commitsChan:       make(chan kyber.Point, 3*n),  // Point FIFO of size 3*n. user-code - receiveCommitments() -> commitsChan -> Get()* - Verifier
		challengeChan:     make(chan kyber.Scalar),      // Scalar unbuffered chan. user-code - receiveChallenges() -> challengeChan ->  PubRand() - Verifier
		subChallengesChan: make(chan []kyber.Scalar),    // []Scalar unbuffered chan. user-code - receiveChallenges() -> challengeChan -> Get() - Verifier
		responsesChan:     make(chan kyber.Scalar, 2*n), // Scalar FIFO of size 2*n. user-code - receiveResponses() -> responsesChan -> Get()* - Verifier
	}
}

// Receive message from Prover
// satisfy the proof.VerifierContext interface, TODO doc, not meant to be used by "user" code see receiveCommitments, receiveCHallenges and receiveResponses methods
func (cvCtx clientVerifierCtx) Get(message interface{}) error {
	switch msg := message.(type) {
	case kyber.Point:
		// Verifier want to receive a commitment
		// receive commitment from user code (via commits channel via receiveCommitments method)
		commitment := <-cvCtx.commitsChan
		msg.Set(commitment)
	case []kyber.Scalar:
		// Verifier want to receive a slice of all n sub-challenges
		// receive sub-challenges from user code (via subChallenges channel via receiveChallenges method)
		// blocks until challenges sent in channel by user code (via receiveChallenges method)
		subChallenges := <-cvCtx.subChallengesChan
		for i, subChallenge := range subChallenges {
			msg[i] = subChallenge // QUESTION use msg[i].Set() instead ?
		}
	case kyber.Scalar:
		// Verifier wants to receive a response
		// receive response from user code (via responses channel via responses method)
		response := <-cvCtx.responsesChan
		msg.Set(response)
	default:
		return errors.New("clientVerifierCtx.Get: message from verifier not of type kyber.Point neither kyber.Scalar nor []kyber.Scalar (" + fmt.Sprintf("%T", message) + ")")
	}
	return nil
}

func (cvCtx clientVerifierCtx) receiveCommitments(commitments []kyber.Point) error {
	if len(commitments) != cap(cvCtx.commitsChan) {
		return errors.New("clientVerifierCtx.receiveCommitments: wrong number of commitments (" +
			strconv.Itoa(len(commitments)) + ") expected " + strconv.Itoa(cap(cvCtx.commitsChan)))
	}

	// Verifier start by calling 3*n times (len(commitments)) Get to obtain the commitments from the Prover
	for _, commit := range commitments {
		cvCtx.commitsChan <- commit // blocks if chan full which never happen (buffer has the right size (3*#clients/predicates in the OrPred))
	}
	return nil
}

func (cvCtx clientVerifierCtx) receiveResponses(responses []kyber.Scalar) error {
	if len(responses) != cap(cvCtx.responsesChan) {
		return errors.New("clientVerifierCtx.receiveResponses: wrong number of responses (" +
			strconv.Itoa(len(responses)) + ") expected " + strconv.Itoa(cap(cvCtx.responsesChan)))
	}

	// Verifier calls Get() 2*n times to obtain the responses
	for _, response := range responses {
		cvCtx.responsesChan <- response // blocks if chan full which never happen (buffer has the right size (2*#clients/predicates in the OrPred))
	}
	return nil
}

func (cvCtx clientVerifierCtx) receiveChallenges(challenge kyber.Scalar, subChallenges []kyber.Scalar) {
	// TODO check length of subChallenges ?
	// TODO FIXME sign/verify ?
	// TODO FIXME here need to check that the challenge in proof is indeed the same that the one that
	// TODO was collectively generated by the servers and sent during proof generation / (sigma-protocol run)
	cvCtx.challengeChan <- challenge
	cvCtx.subChallengesChan <- subChallenges
}

// called by Verifier to obtain the master challenge
// satisfy the proof.VerifierContext interface, TODO doc, not meant to be used by "user" code see receiveChallenges method
func (cvCtx clientVerifierCtx) PubRand(message ...interface{}) error {
	if len(message) != 1 {
		return errors.New("clientVerifierCtx.PubRand called with less or more than one arg, this is not expected")
	}

	// get challenge from user-code (via challenge channel via receiveChallenge method)
	// blocks until challenge received from remote verifier and sent in channel by user code (via receiveChallenge method)
	challenge := <-cvCtx.challengeChan

	switch msg := message[0].(type) {
	case kyber.Scalar:
		msg.Set(challenge)
		return nil
	default:
		return errors.New("clientVerifierCtx.PubRand called with type " + fmt.Sprintf("%T", message[0]) + " instead of kyber.Scalar")
	}
	return nil
}

// Sigma-protocol proof.ProverContext used to conduct interactive proofs with a verifier
// TODO see if can make an interface from my API wrapper to put in kyber.proof
// TODO doc, plus don't use the channels directly there are methods for that
type clientProverCtx struct {
	SuiteProof
	commitsChan       chan kyber.Point    // to extract the prover's commitments from Prover (via Put) and make them accessible (i.e. kind of fix the API...)
	challengeChan     chan kyber.Scalar   // to give master challenge to the Prover (via PubRand) and make them accessible (i.e. kind of fix the API...)
	subChallengesChan chan []kyber.Scalar // to extract the prover's sub-challenges from Prover (via Put) and make them accessible (i.e. kind of fix the API...)
	responsesChan     chan kyber.Scalar   // to extract the prover's responses from Prover (via Put) and make them accessible (i.e. kind of fix the API...)
}

// TODO doc
// n = #clients in auth. group = #predicates in OrProof
func newClientProverCtx(suite Suite, n int) *clientProverCtx {
	// FIXME: see if/where I need to deep copy passed DATA !!
	return &clientProverCtx{
		SuiteProof:        newSuiteProof(suite),
		commitsChan:       make(chan kyber.Point, 3*n),  // Point FIFO of size 3*n. Prover - Put()* -> commitsChan -> commitments() - user-code
		challengeChan:     make(chan kyber.Scalar),      // Scalar unbuffered chan. user-code - receiveChallenge() -> challengeChan -> PubRand() - Prover
		subChallengesChan: make(chan []kyber.Scalar),    // []Scalar unbuffered chan. Prover - Put() -> subChallengesChan -> receiveChallenge() - user-code
		responsesChan:     make(chan kyber.Scalar, 2*n), // Scalar FIFO of size 2*n. Prover - Put()* -> responsesChan -> responses() - user-code
	}
}

// make the Prover's messages available to our/user code
// satisfy the proof.ProverContext interface, TODO doc, not meant to be used by "user" code see commitments, receiveChallenges and responses methods
// QUESTION is there a way/pattern to implement interface with public methods while making them private...? guess no but..
func (cpCtx clientProverCtx) Put(message interface{}) error {
	switch msg := message.(type) {
	case kyber.Point:
		// received message is a commitment
		// send commitment to user code (via commits channel via commitments method)
		cpCtx.commitsChan <- msg // blocks if chan full which should never happen (buffer should have the right size (3*#clients/predicates in the OrPred))
	case []kyber.Scalar:
		// received message is a slice of all n sub-challenges
		// TODO check length correct ? (should be ok if code remains correct..)
		// send sub-challenges to user code (via subChallenges channel via receiveChallenge method)
		cpCtx.subChallengesChan <- msg // blocks until user code received them (sync: "recv happens before send completes")
	case kyber.Scalar:
		// received message is a response
		// send response to user code (via responses channel via responses method)
		cpCtx.responsesChan <- msg // block if chan full which should never happen (buffer should have the right size, (2*#clients/predicates in the OrPred))
	default:
		return errors.New("clientProverCtx.Put: message from prover not of type kyber.Point neither kyber.Scalar nor []kyber.Scalar (" + fmt.Sprintf("%T", message) + ")")
	}
	return nil
}

// retrieve the Prover's first message/commitments t=(t1.0, t1.10, t1.11,..., tn.0, tn.10, tn.11 )
func (cpCtx clientProverCtx) commitments() ([]kyber.Point, error) {
	commitments := make([]kyber.Point, 0, cap(cpCtx.commitsChan))
	for commit := range cpCtx.commitsChan {
		// get commitment from Prover (via commitsChan channel via Put method)
		commitments = append(commitments, commit)
	} // blocks if chan empty (should not be a problem), (and until chan closed by sending side when done (in PubRand()))
	// TODO maybe add a watchdog that will return/log an error if blocked too long  ? (because this should never happen !)
	// TODO in fact good idea but not here, add a test case

	if len(commitments) != cap(cpCtx.commitsChan) {
		return nil, errors.New("clientProverCtx.commitments: received wrong number of commitments (" +
			strconv.Itoa(len(commitments)) + ") expected " + strconv.Itoa(cap(cpCtx.commitsChan)))
	}
	return commitments, nil
}

// retrieve the Prover's responses r=(r1.0, r1.1,..., rn.0, rn.1)
func (cpCtx clientProverCtx) responses() ([]kyber.Scalar, error) {
	responses := make([]kyber.Scalar, 0, cap(cpCtx.responsesChan))
	for response := range cpCtx.responsesChan {
		// get response from Prover (via responsesChan channel via Put method)
		responses = append(responses, response)
	} // blocks if chan empty (should not be a problem), (and until chan closed by sending side when done (when Prover.prove done))
	// TODO maybe add a watchdog that will return an error if blocked too long  ? (because this should never happen !)
	// TODO in fact good idea but not here, add a test case

	if len(responses) != cap(cpCtx.responsesChan) {
		return nil, errors.New("clientProverCtx.responses: received wrong number of responses (" +
			strconv.Itoa(len(responses)) + ") expected " + strconv.Itoa(cap(cpCtx.responsesChan)))
	}
	return responses, nil
}

// Get public randomness / master challenge from verifier/chan
// TODO doc, not meant to be used by "user" code see receiveChallenge method
func (cpCtx clientProverCtx) PubRand(message ...interface{}) error {
	if len(message) != 1 {
		return errors.New("clientProverCtx.PubRand called with less or more than one arg, this is not expected")
	}
	// Prover is done sending the commits with Put => release sync barrier with commitments() method by closing the channel
	close(cpCtx.commitsChan)

	// get challenge from remote verifier (via challenge channel via receiveChallenge method)
	// blocks until challenge received from remote verifier and sent in channel by user code (via receiveChallenge method)
	challenge := <-cpCtx.challengeChan

	switch scalar := message[0].(type) {
	case kyber.Scalar:
		scalar.Set(challenge)
		return nil
	default:
		return errors.New("clientProverCtx.PubRand called with type " + fmt.Sprintf("%T", message[0]) + " instead of kyber.Scalar")
	}
}

// send master challenge to Prover
// TODO doc
func (cpCtx clientProverCtx) receiveChallenges(challenge kyber.Scalar) []kyber.Scalar {
	// send master challenge to Prover (via challenge channel via PubRand method) => release sync barrier with PubRand()
	cpCtx.challengeChan <- challenge // blocks until Prover received the master challenge (sync: "recv happens before send completes")

	// receive sub-challenges
	subChallenges := <-cpCtx.subChallengesChan
	return subChallenges
}

// Get private randomness
// TODO doc, not meant to be used by "user" code
func (cpCtx clientProverCtx) PriRand(message ...interface{}) error {
	if len(message) > 0 {
		switch scalar := message[0].(type) {
		case kyber.Scalar:
			scalar.Pick(cpCtx.RandomStream())
		default:
			return errors.New("clientProverCtx.PriRand called with type " + fmt.Sprintf("%T", message[0]) + " instead of kyber.Scalar")
		}
	}
	return errors.New("clientProverCtx.PriRand called with no arg, this is not expected")
}

/*clientProof stores the client's proof P as of "Syta - Identity Management Through Privacy Preserving Aut 4.3.7"
 */
// TODO doc
type clientProof struct {
	cs kyber.Scalar
	t  []kyber.Point
	c  []kyber.Scalar
	r  []kyber.Scalar
}

// TODO FIXME maybe, pack the 2 channels in a new type and add methods NewTestProxy NewProxy etc..
// for now only add two channels to the newClientProof function
//type serverProxy struct {
//	send
//}
// FIXME names and interface
// TODO decide communication with server
// TODO lambdas/callerpassedclosures higherorder functions whatever to call to communicate with remote server
// TODO doc, build the clientProof (as of DAGA paper) and return it to caller
func newClientProof(context authenticationContext,
	client Client,
	tagAndCommitments initialTagAndCommitments,
	s kyber.Scalar,
	pushCommitments chan<- []kyber.Point,
	pullChallenge <-chan Challenge) (clientProof, error) {
	//construct the proof.Prover for client's PK and its proof.ProverContext
	prover := newClientProver(context, tagAndCommitments, client, s)
	proverCtx := newClientProverCtx(suite, len(context.g.x))

	if len(context.g.x) <= 1 {
		return clientProof{}, errors.New("there is only one client in the context, this means DAGA is pointless")
		// moreover the following code (and more or less DAGA paper) assumes that there is at least 2 clients/predicates
		// in the context/OR-predicate, if this condition is not met there won't be an "subChallenges" to generate by the
		// prover => he won't send them by calling Put, but we wait for them !!
		// in case this assumption needs to be relaxed, a test should be added to the proverContext.receiveChallenges() method
	}

	//3-move interaction with server
	//	start the proof.Prover and proof machinery in new goroutine
	var P clientProof
	var proverErr error
	go func() {
		defer close(proverCtx.responsesChan)
		proverErr = prover(proverCtx)
	}()

	//	get initial commitments from running Prover
	if commits, err := proverCtx.commitments(); err != nil {
		return clientProof{}, err
	} else {
		P.t = commits
	}

	// TODO pack those in a single step and maybe remove these channels and instead call userprovided function "sendCommitsReceiveChallenge"
	// TODO but keep in mind that cannot accept challenge as parameter if asked by someone.. first the commitments should be sent to the servers
	//	forward them to random remote server/verifier (over *anon.* circuit etc.. concern of the caller code !!)
	pushCommitments <- P.t
	//	receive master challenge from remote server (over *anon.* circuit etc.. concern of the caller code !!)
	challenge := <-pullChallenge
	if err := challenge.verifySignatures(context.g.y); err != nil {
		// TODO log
		return clientProof{}, err
	}
	P.cs = challenge.cs

	//	forward master challenge to running Prover in order to continue the proof process, and receive the sub-challenges from Prover
	P.c = proverCtx.receiveChallenges(P.cs)

	//	get final responses from Prover
	if responses, err := proverCtx.responses(); err != nil {
		// TODO onet.log something
		return clientProof{}, err
	} else {
		P.r = responses
	}

	//check return value of the now done proof.Prover
	if proverErr != nil { // here no race, we are sure that Prover is done since responses() returns only after response chan is closed
		// TODO onet.log something
		return clientProof{}, proverErr
	}
	return P, nil
}

// TODO doc + return type...
/*verifyClientProof checks the validity of a client's proof*/
func verifyClientProof(context authenticationContext,
	proof clientProof,
	tagAndCommitments initialTagAndCommitments) bool {
	//construct the proof.Verifier for client's PK and its proof.VerifierContext
	verifier := newClientVerifier(context, tagAndCommitments)
	verifierCtx := newClientVerifierCtx(suite, len(context.g.x))

	//3-move interaction with client
	//	start the proof.Verifier and proof machinery in new goroutine
	// TODO package this in a function
	verifierErrChan := make(chan error)
	go func() {
		verifierErrChan <- verifier(verifierCtx)
	}()

	//	forward commitments to running Verifier
	commitments := proof.t
	if err := verifierCtx.receiveCommitments(commitments); err != nil {
		// TODO log
		return false
	}

	//	forward challenges to running Verifier
	challenge := proof.cs
	subChallenges := proof.c
	verifierCtx.receiveChallenges(challenge, subChallenges)

	//	forward responses to running Verifier
	responses := proof.r
	if err := verifierCtx.receiveResponses(responses); err != nil {
		// TODO log
		return false
	}

	//wait for Verifier to be done and check return value of the now done proof.Verifier
	verifierErr := <-verifierErrChan
	if verifierErr != nil { // here no race, we are sure that Verifier is done since responses() returns only after response chan is closed
		// TODO log
		return false
	}
	return true
}

// TODO doc
func newClientProofPred(context authenticationContext, tagAndCommitments initialTagAndCommitments) (proof.Predicate, map[string]kyber.Point) {
	// build the OR-predicate
	andPreds := make([]proof.Predicate, 0, len(context.g.x))

	// map for public values needed to construct the Prover and Verifier from the predicate
	pval := make(map[string]kyber.Point, 3+2*len(context.g.x))
	pval["G"] = suite.Point().Base()
	pval["T0"] = tagAndCommitments.t0
	pval["Sm"] = tagAndCommitments.sCommits[len(tagAndCommitments.sCommits)-1]

	//	build all the internal And predicates (one for each client in current auth. group)
	for k, pubKey := range context.g.x {
		// client AndPred
		kStr := strconv.Itoa(k)
		//		i) client i’s linkage tag T0 is created with respect to his per-round generator hi
		linkageTagValidPred := proof.Rep("T0", "s", "H"+kStr)
		// 		ii)  S is a proper commitment to the product of all secrets that i shares with the servers
		commitmentValidPred := proof.Rep("Sm", "s", "G")
		// 		iii) client i's private key xi corresponds to one of the public keys included in the group definition G
		knowOnePrivateKeyPred := proof.Rep("X"+kStr, "x"+kStr, "G")

		clientAndPred := proof.And(linkageTagValidPred, commitmentValidPred, knowOnePrivateKeyPred)

		// update map of public values
		pval["X"+kStr] = pubKey
		pval["H"+kStr] = context.h[k]

		andPreds = append(andPreds, clientAndPred)
	}
	finalOrPred := proof.Or(andPreds...)

	return finalOrPred, pval
}

// TODO doc, + see if can clean/lift a little the parameters + QUESTION move where I'll put server related functions and structs since it will be the servers that will use it or package everything related to client proof in same file ??
func newClientVerifier(context authenticationContext, tagAndCommitments initialTagAndCommitments) proof.Verifier {
	// build OR-predicate of the client proof, and the map of public values
	finalOrPred, pval := newClientProofPred(context, tagAndCommitments)

	// retrieve sigma-protocol Verifier for the OR-predicate
	return finalOrPred.Verifier(newSuiteProof(suite), pval)
}

// TODO doc, + see if can clean/lift a little the parameters, + see if context big enough to make it a pointer
func newClientProver(context authenticationContext, tagAndCommitments initialTagAndCommitments, client Client, s kyber.Scalar) proof.Prover {
	// build OR-predicate of the client proof, and the map of public values
	finalOrPred, pval := newClientProofPred(context, tagAndCommitments)

	// build map of secret values and choice needed to construct the Prover from the predicate
	choice := make(map[proof.Predicate]int, 1)
	sval := make(map[string]kyber.Scalar, 2)
	sval["s"] = s
	sval["x"+strconv.Itoa(client.index)] = client.key.Private
	choice[finalOrPred] = client.index // indicate to prover which clause is actually true

	// retrieve sigma-protocol Prover for the OR-predicate
	prover := finalOrPred.Prover(newSuiteProof(suite), sval, pval, choice)
	return prover
}
