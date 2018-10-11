package daga

import (
	"crypto/sha512"
	"github.com/dedis/kyber"
	"github.com/stretchr/testify/assert"
	"io"
	"math/rand"
	"testing"
)

func TestNewServer(t *testing.T) {
	//Normal execution
	i := rand.Int()
	s := suite.Scalar().Pick(suite.RandomStream())
	server, err := NewServer(i, s)
	assert.NoError(t, err, "Cannot initialize a new server with a given private key")
	assert.Equal(t, server.index, i, "Cannot initialize a new server with a given private key, wrong index")
	assert.True(t, server.key.Private.Equal(s), "Cannot initialize a new server with a given private key, wrong key")

	server, err = NewServer(i, nil)
	assert.NoError(t, err, "Cannot create a new server without a private key")
	assert.NotNil(t, server.key.Private, "Cannot create a new server without a private key")

	//Invalid input
	server, err = NewServer(-2, s)
	assert.Error(t, err, "Wrong check: Invalid index")
}

func TestGetPublicKey_Server(t *testing.T) {
	server, _ := NewServer(0, suite.Scalar().Pick(suite.RandomStream()))
	P := server.PublicKey()
	assert.NotNil(t, P, "Cannot get public key")
}

func TestGenerateCommitment(t *testing.T) {
	_, servers, context, _ := GenerateTestContext(rand.Intn(10)+2, rand.Intn(10)+1)

	//Normal execution
	commit, opening, err := servers[0].GenerateCommitment(context)
	assert.NoError(t, err, "Cannot generate a commitment")
	assert.True(t, commit.commit.Equal(suite.Point().Mul(opening, nil)), "Cannot open the commitment")

	msg, err := commit.commit.MarshalBinary()
	assert.NoError(t, err, "failed to marshall commitment")

	err = SchnorrVerify(servers[0].PublicKey(), msg, commit.sig.sig)
	assert.NoError(t, err, "wrong commitment signature, failed to verify")
}

func TestVerifyCommitmentSignature(t *testing.T) {
	_, servers, context, _ := GenerateTestContext(rand.Intn(10)+1, rand.Intn(10)+1)

	//Generate commitments
	var commits []Commitment
	for _, server := range servers {
		commit, _, _ := server.GenerateCommitment(context)
		commits = append(commits, *commit)
	}

	//Normal execution
	err := VerifyCommitmentSignature(context, commits)
	assert.NoError(t, err, "Cannot verify the signatures for a legit commit array")

	//Change a random index
	i := rand.Intn(len(servers))
	commits[i].sig.index = i + 1
	err = VerifyCommitmentSignature(context, commits)
	assert.Error(t, err, "Cannot verify matching indexes for %d", i)

	commits[i].sig.index = i + 1

	//Change a signature
	//Code shown as not covered, but it does detect the modification and returns an error <- QUESTION ??
	sig := commits[i].sig.sig
	sig = append(sig, []byte("A")...)
	sig = sig[1:]
	commits[i].sig.sig = sig
	err = VerifyCommitmentSignature(context, commits)
	assert.Error(t, err, "Cannot verify signature for %d", i)
}

func TestCheckOpenings(t *testing.T) {
	_, servers, context, _ := GenerateTestContext(rand.Intn(10)+1, rand.Intn(10)+1)

	//Generate commitments
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}

	//Normal execution
	cs, err := CheckOpenings(context, commits, openings)
	assert.NoError(t, err, "Cannot check the openings")

	challenge := suite.Scalar().Zero()
	for _, temp := range openings {
		challenge = suite.Scalar().Add(challenge, temp)
	}
	assert.True(t, cs.Equal(challenge), "Wrong computation of challenge cs: %s instead of %s", cs, challenge)

	//Empty inputs
	cs, err = CheckOpenings(nil, commits, openings)
	assert.Error(t, err, "Wrong check: Empty context")

	assert.Nil(t, cs, "cs not nil on empty context")

	cs, err = CheckOpenings(context, nil, openings)
	assert.Error(t, err, "Wrong check: Empty commits")
	assert.Nil(t, cs, "cs not nil on empty commits")

	cs, err = CheckOpenings(context, commits, nil)
	assert.Error(t, err, "Wrong check: Empty openings")
	assert.Nil(t, cs, "cs not nil on empty openings")

	//Change the length of the openings
	CutOpenings := openings[:len(openings)-1]
	cs, err = CheckOpenings(context, commits, CutOpenings)
	assert.Error(t, err, "Invalid length check on openings")
	assert.Nil(t, cs, "cs not nil on opening length error")

	//Change the length of the commits
	CutCommits := commits[:len(commits)-1]
	cs, err = CheckOpenings(context, CutCommits, openings)
	assert.Error(t, err, "Invalid length check on comits")
	assert.Nil(t, cs, "cs not nil on commit length error")

	//Change a random opening
	i := rand.Intn(len(servers))
	openings[i] = suite.Scalar().Zero()
	cs, err = CheckOpenings(context, commits, openings)
	assert.Error(t, err, "Invalid opening check")
	assert.Nil(t, cs, "cs not nil on opening error")
}

func TestInitializeChallenge(t *testing.T) {
	_, servers, context, _ := GenerateTestContext(rand.Intn(10)+1, rand.Intn(10)+1)

	//Generate commitments
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}

	//Normal execution
	challenge, err := InitializeChallenge(context, commits, openings)
	assert.NoError(t, err, "Cannot initialize challenge")
	assert.NotNil(t, challenge, "Cannot initialize challenge")

	//Empty inputs
	challenge, err = InitializeChallenge(nil, commits, openings)
	assert.Error(t, err, "Wrong check: Empty cs")
	assert.Nil(t, challenge, "Wrong check: Empty cs")

	challenge, err = InitializeChallenge(context, nil, openings)
	assert.Error(t, err, "Wrong check: Empty commits")
	assert.Nil(t, challenge, "Wrong check: Empty commits")

	challenge, err = InitializeChallenge(context, commits, nil)
	assert.Error(t, err, "Wrong check: Empty openings")
	assert.Nil(t, challenge, "Wrong check: Empty openings")

	//Mismatch length between commits and openings
	challenge, err = InitializeChallenge(context, commits, openings[:len(openings)-2])
	assert.Error(t, err, "Wrong check: Mismatched length between commits and openings")
	assert.Nil(t, challenge, "Wrong check: Mismatched length between commits and openings")

	//Change an opening
	openings[0] = suite.Scalar().Zero()
	challenge, err = InitializeChallenge(context, commits, openings[:len(openings)-2])
	assert.Error(t, err, "Invalid opening check")
	assert.Nil(t, challenge, "Invalid opening check")
}

func TestCheckUpdateChallenge(t *testing.T) {
	//The following tests need at least 2 servers
	_, servers, context, _ := GenerateTestContext(rand.Intn(10)+1, rand.Intn(10)+2)

	//Generate commitments
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}

	challenge, _ := InitializeChallenge(context, commits, openings)
	cs := challenge.cs

	//Normal execution
	err := servers[0].CheckUpdateChallenge(context, challenge)
	assert.NoError(t, err, "Cannot update the challenge")
	assert.Equal(t, len(challenge.sigs), 1, "Did not correctly add the signature")

	//Duplicate signature
	challenge.sigs = append(challenge.sigs, challenge.sigs[0])
	err = servers[0].CheckUpdateChallenge(context, challenge)
	assert.Error(t, err, "Does not check for duplicates signatures")

	challenge.sigs = []serverSignature{challenge.sigs[0]}

	//Altered signature
	fake := append([]byte("A"), challenge.sigs[0].sig...)
	challenge.sigs[0].sig = fake[:len(challenge.sigs[0].sig)]
	err = servers[0].CheckUpdateChallenge(context, challenge)
	assert.Error(t, err, "Wrond check of signature")

	//Restore correct signature for the next tests
	challenge.sigs = nil
	servers[0].CheckUpdateChallenge(context, challenge)

	//Modify the challenge
	challenge.cs = suite.Scalar().Zero()
	err = servers[0].CheckUpdateChallenge(context, challenge)
	assert.Error(t, err, "Does not check the challenge")

	challenge.cs = cs

	//Only appends if the challenge has not already done a round-robin
	for _, server := range servers[1:] {
		err = server.CheckUpdateChallenge(context, challenge)
		assert.NoError(t, err, "Error during the round-robin at server %d", server.index)
	}
	err = servers[0].CheckUpdateChallenge(context, challenge)
	assert.NoError(t, err, "Error when closing the loop of the round-robin")
	assert.Equal(t, len(challenge.sigs), len(servers), "Invalid number of signatures: %d instead of %d", len(challenge.sigs), len(servers))

	//Change a commitment
	challenge.commits[0].commit = suite.Point().Mul(suite.Scalar().One(), nil)
	err = servers[0].CheckUpdateChallenge(context, challenge)
	assert.Error(t, err, "Invalid commitment signature check")

	challenge.commits[0].commit = suite.Point().Mul(challenge.openings[0], nil)

	//Change an opening
	challenge.openings[0] = suite.Scalar().Zero()
	err = servers[0].CheckUpdateChallenge(context, challenge)
	assert.Error(t, err, "Invalid opening check")
}

func TestFinalizeChallenge(t *testing.T) {
	//The following tests need at least 2 servers
	_, servers, context, _ := GenerateTestContext(rand.Intn(10)+1, rand.Intn(10)+2)

	//Generate commitments
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}

	challenge, _ := InitializeChallenge(context, commits, openings)

	//Makes every server update the challenge
	var err error
	for _, server := range servers[1:] {
		err = server.CheckUpdateChallenge(context, challenge)
		assert.NoError(t, err, "Error during the round-robin at server %d", server.index)
	}

	//Normal execution
	//Let's say that server 0 is the leader and received the message back
	servers[0].CheckUpdateChallenge(context, challenge)
	clientChallenge, err := FinalizeChallenge(context, challenge)
	assert.NoError(t, err, "Error during finalization of the challenge")

	//Check cs value
	assert.True(t, clientChallenge.cs.Equal(challenge.cs), "cs values does not match")

	//Check number of signatures
	assert.Equal(t, len(clientChallenge.sigs), len(challenge.sigs), "Signature count does not match: got %d expected %d", len(clientChallenge.sigs), len(challenge.sigs))

	//Empty inputs
	clientChallenge, err = FinalizeChallenge(nil, challenge)
	assert.Error(t, err, "Wrong check: Empty context")
	assert.Zero(t, clientChallenge, "Wrong check: Empty context")

	clientChallenge, err = FinalizeChallenge(context, nil)
	assert.Error(t, err, "Wrong check: Empty challenge")
	assert.Zero(t, clientChallenge, "Wrong check: Empty challenge")

	//Add a signature
	challenge.sigs = append(challenge.sigs, challenge.sigs[0])
	clientChallenge, err = FinalizeChallenge(context, challenge)
	assert.Error(t, err, "Wrong check: Higher signature count")
	assert.Zero(t, clientChallenge, "Wrong check: Higher signature count")

	//Remove a signature
	challenge.sigs = challenge.sigs[:len(challenge.sigs)-2]
	clientChallenge, err = FinalizeChallenge(context, challenge)
	assert.Error(t, err, "Wrong check: Lower signature count")
	assert.Zero(t, clientChallenge, "Wrong check: Lower signature count")
}

// TODO port to new implementation
func TestInitializeServerMessage(t *testing.T) {
	// TODO test for one server as we saw that it previously triggered an hidden bug
	clients, servers, context, _ := GenerateTestContext(2, 2)
	for _, server := range servers {
		if server.r == nil {
			t.Errorf("Error in r for server %d", server.index)
		}
	}
	tagAndCommitments, s := newInitialTagAndCommitments(context.g.y, context.h[clients[0].index])

	//Generate a valid challenge
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}
	challenge, _ := InitializeChallenge(context, commits, openings)
	//Sign the challenge
	for _, server := range servers {
		server.CheckUpdateChallenge(context, challenge)
	}
	clientChallenge, _ := FinalizeChallenge(context, challenge)
	// setup test server "channels"
	pushCommitments, pullChallenge := newDummyServerChannels(clientChallenge)

	//Assemble the client message
	proof, err := newClientProof(*context, clients[0], *tagAndCommitments, s, pushCommitments, pullChallenge)
	assert.NoError(t, err, "failed to generate client proof, this is not expected")
	clientMessage := authenticationMessage{
		c:                        *context,
		initialTagAndCommitments: *tagAndCommitments,
		p0:                       proof,
	}

	//Normal execution
	servMsg := servers[0].InitializeServerMessage(&clientMessage)
	if servMsg == nil || len(servMsg.indexes) != 0 || len(servMsg.proofs) != 0 || len(servMsg.tags) != 0 || len(servMsg.sigs) != 0 {
		t.Error("Cannot initialize server message")
	}

	//Empty request
	servMsg = servers[0].InitializeServerMessage(nil)
	assert.Nil(t, servMsg, "Wrong check: Empty request")
}

func TestServerProtocol(t *testing.T) {
	clients, servers, context, _ := GenerateTestContext(2, 2)
	for _, server := range servers {
		assert.NotNil(t, server.r, "Error in r for server %d", server.index)
	}
	tagAndCommitments, s := newInitialTagAndCommitments(context.g.y, context.h[clients[0].index])

	//Generate a valid challenge
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}
	challenge, _ := InitializeChallenge(context, commits, openings)
	//Sign the challenge
	for _, server := range servers {
		server.CheckUpdateChallenge(context, challenge)
	}
	clientChallenge, _ := FinalizeChallenge(context, challenge)

	// setup test server "channels"
	pushCommitments, pullChallenge := newDummyServerChannels(clientChallenge)

	//Assemble the client message
	proof, err := newClientProof(*context, clients[0], *tagAndCommitments, s, pushCommitments, pullChallenge)
	assert.NoError(t, err, "failed to generate client proof, this is not expected")
	clientMessage := authenticationMessage{
		c:                        *context,
		initialTagAndCommitments: *tagAndCommitments,
		p0:                       proof,
	}

	//Original hash for later test
	hasher := sha512.New()
	var writer io.Writer = hasher
	data, _ := clientMessage.ToBytes()
	writer.Write(data)
	hash := hasher.Sum(nil)

	//Create the initial server message
	servMsg := ServerMessage{request: clientMessage, proofs: nil, tags: nil, sigs: nil, indexes: nil}

	//Normal execution for correct client
	err = servers[0].ServerProtocol(context, &servMsg)
	assert.NoError(t, err, "Error in Server Protocol\n%s", err)

	err = servers[1].ServerProtocol(context, &servMsg)
	assert.NoError(t, err, "Error in Server Protocol for server 1\n%s", err)

	//Check that elements were added to the message
	assert.Equal(t, 2, len(servMsg.indexes), "Incorrect number of elements added to the message: %d instead of 2", len(servMsg.indexes))

	//Empty request
	emptyMsg := ServerMessage{request: authenticationMessage{}, proofs: servMsg.proofs, tags: servMsg.tags, sigs: servMsg.sigs, indexes: servMsg.indexes}
	err = servers[0].ServerProtocol(context, &emptyMsg)
	assert.Error(t, err, "Wrong check: Empty request")

	//Different lengths
	wrongMsg := ServerMessage{request: clientMessage, proofs: servMsg.proofs, tags: servMsg.tags, sigs: servMsg.sigs, indexes: servMsg.indexes}
	wrongMsg.indexes = wrongMsg.indexes[:len(wrongMsg.indexes)-2]
	err = servers[0].ServerProtocol(context, &wrongMsg)
	assert.Error(t, err, "Wrong check: different field length of indexes")

	wrongMsg = ServerMessage{request: clientMessage, proofs: servMsg.proofs, tags: servMsg.tags, sigs: servMsg.sigs, indexes: servMsg.indexes}
	wrongMsg.tags = wrongMsg.tags[:len(wrongMsg.tags)-2]
	err = servers[0].ServerProtocol(context, &wrongMsg)
	assert.Error(t, err, "Wrong check: different field length of tags")

	wrongMsg = ServerMessage{request: clientMessage, proofs: servMsg.proofs, tags: servMsg.tags, sigs: servMsg.sigs, indexes: servMsg.indexes}
	wrongMsg.proofs = wrongMsg.proofs[:len(wrongMsg.proofs)-2]
	err = servers[0].ServerProtocol(context, &wrongMsg)
	assert.Error(t, err, "Wrong check: different field length of proofs")

	wrongMsg = ServerMessage{request: clientMessage, proofs: servMsg.proofs, tags: servMsg.tags, sigs: servMsg.sigs, indexes: servMsg.indexes}
	wrongMsg.sigs = wrongMsg.sigs[:len(wrongMsg.sigs)-2]
	err = servers[0].ServerProtocol(context, &wrongMsg)
	assert.Error(t, err, "Wrong check: different field length of signatures")

	//Modify the client proof
	wrongClient := ServerMessage{request: clientMessage, proofs: servMsg.proofs, tags: servMsg.tags, sigs: servMsg.sigs, indexes: servMsg.indexes}
	wrongClient.request.p0 = clientProof{}
	err = servers[0].ServerProtocol(context, &wrongMsg)
	assert.Error(t, err, "Wrong check: invalid client proof")

	//Too many calls
	err = servers[0].ServerProtocol(context, &servMsg)
	assert.Error(t, err, "Wrong check: Too many calls")

	//The client request is left untouched
	hasher2 := sha512.New()
	var writer2 io.Writer = hasher2
	data2, _ := servMsg.request.ToBytes()
	writer2.Write(data2)
	hash2 := hasher2.Sum(nil)

	for i := range hash {
		assert.Equal(t, hash[i], hash2[i], "Client's request modified")
	}

	//Normal execution for misbehaving client
	misbehavingMsg := ServerMessage{request: clientMessage, proofs: nil, tags: nil, sigs: nil, indexes: nil}
	misbehavingMsg.request.sCommits[2] = suite.Point().Null() //change the commitment for server 0
	err = servers[0].ServerProtocol(context, &misbehavingMsg)
	assert.NoError(t, err, "Error in Server Protocol for misbehaving client\n%s", err)

	err = servers[1].ServerProtocol(context, &misbehavingMsg)
	assert.NoError(t, err, "Error in Server Protocol for misbehaving client and server 1\n%s", err)
}

func TestGenerateServerProof(t *testing.T) {
	clients, servers, context, _ := GenerateTestContext(2, 2)
	tagAndCommitments, s := newInitialTagAndCommitments(context.g.y, context.h[clients[0].index])
	T0, _ := tagAndCommitments.t0, tagAndCommitments.sCommits

	//Generate a valid challenge
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}
	challenge, _ := InitializeChallenge(context, commits, openings)
	//Sign the challenge
	for _, server := range servers {
		server.CheckUpdateChallenge(context, challenge)
	}
	clientChallenge, _ := FinalizeChallenge(context, challenge)
	// setup test server "channels"
	pushCommitments, pullChallenge := newDummyServerChannels(clientChallenge)

	//Assemble the client message
	proof, err := newClientProof(*context, clients[0], *tagAndCommitments, s, pushCommitments, pullChallenge)
	assert.NoError(t, err, "failed to generate client proof, this is not expected")
	clientMessage := authenticationMessage{
		c:                        *context,
		initialTagAndCommitments: *tagAndCommitments,
		p0:                       proof,
	}

	//Create the initial server message
	servMsg := ServerMessage{request: clientMessage, proofs: nil, tags: nil, sigs: nil, indexes: nil}

	//Prepare the proof
	hasher := sha512.New()
	var writer io.Writer = hasher // ...
	suite.Point().Mul(servers[0].key.Private, servMsg.request.sCommits[0]).MarshalTo(writer)
	hash := hasher.Sum(nil)
	hasher = suite.Hash()
	hasher.Write(hash)
	//rand := suite.Cipher(hash)
	secret := suite.Scalar().SetBytes(hasher.Sum(nil))

	inv := suite.Scalar().Inv(secret)
	exp := suite.Scalar().Mul(servers[0].r, inv)
	T := suite.Point().Mul(exp, T0)

	//Normal execution
	serverProof, err := servers[0].generateServerProof(context, secret, T, &servMsg)
	assert.NoError(t, err, "Cannot generate normal server proof")
	assert.NotNil(t, serverProof, "Cannot generate normal server proof")

	//Correct format
	if serverProof.t1 == nil || serverProof.t2 == nil || serverProof.t3 == nil {
		t.Error("Incorrect tags in proof")
	}
	assert.NotNil(t, serverProof.c, "Incorrect challenge")

	assert.NotNil(t, serverProof.r1, "Incorrect responses")
	assert.NotNil(t, serverProof.r2, "Incorrect responses")

	//Invalid inputs
	serverProof, err = servers[0].generateServerProof(nil, secret, T, &servMsg)
	assert.Error(t, err, "Wrong check: Invalid context")
	assert.Nil(t, serverProof, "Wrong check: Invalid context")

	serverProof, err = servers[0].generateServerProof(context, nil, T, &servMsg)
	assert.Error(t, err, "Wrong check: Invalid secret")
	assert.Nil(t, serverProof, "Wrong check: Invalid secret")

	serverProof, err = servers[0].generateServerProof(context, secret, nil, &servMsg)
	assert.Error(t, err, "Wrong check: Invalid tag")
	assert.Nil(t, serverProof, "Wrong check: Invalid tag")

	serverProof, err = servers[0].generateServerProof(context, secret, T, nil)
	assert.Error(t, err, "Wrong check: Invalid Server Message")
	assert.Nil(t, serverProof, "Wrong check: Invalid Server Message")
}

func TestVerifyServerProof(t *testing.T) {
	clients, servers, context, _ := GenerateTestContext(2, rand.Intn(10)+2)
	tagAndCommitments, s := newInitialTagAndCommitments(context.g.y, context.h[clients[0].index])

	//Generate a valid challenge
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}
	challenge, _ := InitializeChallenge(context, commits, openings)
	//Normal execution
	for _, server := range servers {
		server.CheckUpdateChallenge(context, challenge)
	}
	clientChallenge, _ := FinalizeChallenge(context, challenge)
	// setup test server "channels"
	pushCommitments, pullChallenge := newDummyServerChannels(clientChallenge)

	//Assemble the client message
	clientProof, err := newClientProof(*context, clients[0], *tagAndCommitments, s, pushCommitments, pullChallenge)
	assert.NoError(t, err, "failed to generate client proof, this is not expected")
	clientMessage := authenticationMessage{
		c:                        *context,
		initialTagAndCommitments: *tagAndCommitments,
		p0:                       clientProof,
	}

	servMsg := ServerMessage{request: clientMessage, proofs: nil, tags: nil, sigs: nil, indexes: nil}

	err = servers[0].ServerProtocol(context, &servMsg)
	assert.NoError(t, err)
	// TODO, I replaced the commented code below by the call above (which is perfectly sound) but this triggers new questions,
	// TODO => serverprotocol => verifyserverproof, reorganize tests or rewrite everything to follow testing guidelines or make sure everything is in the right place
	////Prepare the proof
	//hasher := suite.Hash()
	//suite.Point().Mul(servers[0].key.Private, servMsg.request.sCommits[0]).MarshalTo(hasher)
	////rand := suite.Cipher(hash)
	//secret := suite.Scalar().SetBytes(hasher.Sum(nil))
	//
	//inv := suite.Scalar().Inv(secret)
	//exp := suite.Scalar().Mul(servers[0].r, inv)
	//T := suite.Point().Mul(exp, tagAndCommitments.t0)
	//
	////Generate the proof
	//proof, _ := servers[0].generateServerProof(context, secret, T, &servMsg)
	//servMsg.proofs = append(servMsg.proofs, *proof)
	//servMsg.tags = append(servMsg.tags, T)
	//servMsg.indexes = append(servMsg.indexes, servers[0].index)
	//
	////Signs our message
	//data, _ := servMsg.request.ToBytes()
	//temp, _ := T.MarshalBinary()
	//data = append(data, temp...)
	//temp, _ = proof.ToBytes()
	//data = append(data, temp...)
	//data = append(data, []byte(strconv.Itoa(servers[0].index))...)
	//sign, _ := ECDSASign(servers[0].key.Private, data)
	//signature := serverSignature{sig: sign, index: servers[0].index}
	//servMsg.sigs = append(servMsg.sigs, signature)
	//
	////Verify first server proof
	//check := verifyServerProof(context, 0, &servMsg)
	//if !check {
	//	t.Error("Cannot verify first valid normal server proof")
	//}

	err = servers[1].ServerProtocol(context, &servMsg)
	assert.NoError(t, err)
	// FIXME why invalid signature ?? seems here is another error in previous code

	//Verify any server proof
	check := verifyServerProof(context, 1, &servMsg)
	assert.True(t, check, "Cannot verify valid normal server proof")

	saveProof := serverProof{c: servMsg.proofs[1].c,
		t1: servMsg.proofs[1].t1,
		t2: servMsg.proofs[1].t2,
		t3: servMsg.proofs[1].t3,
		r1: servMsg.proofs[1].r1,
		r2: servMsg.proofs[1].r2,
	}

	//Check inputs
	servMsg.proofs[1].c = nil
	check = verifyServerProof(context, 1, &servMsg)
	assert.False(t, check, "Error in challenge verification")
	servMsg.proofs[1].c = saveProof.c

	servMsg.proofs[1].t1 = nil
	check = verifyServerProof(context, 1, &servMsg)
	assert.False(t, check, "Error in t1 verification")
	servMsg.proofs[1].t1 = saveProof.t1

	servMsg.proofs[1].t2 = nil
	check = verifyServerProof(context, 1, &servMsg)
	assert.False(t, check, "Error in t2 verification")
	servMsg.proofs[1].t2 = saveProof.t2

	servMsg.proofs[1].t3 = nil
	check = verifyServerProof(context, 1, &servMsg)
	assert.False(t, check, "Error in t3 verification")
	servMsg.proofs[1].t3 = saveProof.t3

	servMsg.proofs[1].r1 = nil
	check = verifyServerProof(context, 1, &servMsg)
	assert.False(t, check, "Error in r1 verification")
	servMsg.proofs[1].r1 = saveProof.r1

	servMsg.proofs[1].r2 = nil
	check = verifyServerProof(context, 1, &servMsg)
	assert.False(t, check, "Error in r2 verification")
	servMsg.proofs[1].r2 = saveProof.r2

	//Invalid context
	check = verifyServerProof(nil, 1, &servMsg)
	assert.False(t, check, "Wrong check: Invalid context")

	//nil message
	check = verifyServerProof(context, 1, nil)
	assert.False(t, check, "Wrong check: Invalid message")

	//Invalid value of i
	check = verifyServerProof(context, 2, &servMsg)
	assert.False(t, check, "Wrong check: Invalid i value")

	check = verifyServerProof(context, -2, &servMsg)
	assert.False(t, check, "Wrong check: Negative i value")
}

func TestGenerateMisbehavingProof(t *testing.T) {
	clients, servers, context, _ := GenerateTestContext(2, 2)
	tagAndCommitments, s := newInitialTagAndCommitments(context.g.y, context.h[clients[0].index])

	//Generate a valid challenge
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}

	challenge, _ := InitializeChallenge(context, commits, openings)

	//Generate the challenge
	for _, server := range servers {
		server.CheckUpdateChallenge(context, challenge)
	}

	clientChallenge, _ := FinalizeChallenge(context, challenge)

	// setup test server "channels"
	pushCommitments, pullChallenge := newDummyServerChannels(clientChallenge)

	//Assemble the client message
	proof, err := newClientProof(*context, clients[0], *tagAndCommitments, s, pushCommitments, pullChallenge)
	assert.NoError(t, err, "failed to generate client proof, this is not expected")
	clientMessage := authenticationMessage{
		c:                        *context,
		initialTagAndCommitments: *tagAndCommitments,
		p0:                       proof,
	}

	serverProof, err := servers[0].generateMisbehavingProof(context, clientMessage.sCommits[0])
	if err != nil || serverProof == nil {
		t.Error("Cannot generate misbehaving proof")
	}

	//Correct format
	assert.NotNil(t, serverProof.t1, "t1 nil for misbehaving proof")
	assert.NotNil(t, serverProof.t2, "t2 nil for misbehaving proof")
	assert.NotNil(t, serverProof.t3, "t3 nil for misbehaving proof")
	assert.NotNil(t, serverProof.c, "c nil for misbehaving proof")
	assert.NotNil(t, serverProof.r1, "r1 nil for misbehaving proof")
	assert.Nil(t, serverProof.r2, "r2 not nil for misbehaving proof")

	//Invalid inputs
	serverProof, err = servers[0].generateMisbehavingProof(nil, clientMessage.sCommits[0])
	assert.Error(t, err, "Wrong check: Invalid context")
	assert.Nil(t, serverProof, "Wrong check: Invalid context")

	serverProof, err = servers[0].generateMisbehavingProof(context, nil)
	assert.Error(t, err, "Wrong check: Invalid Z")
	assert.Nil(t, serverProof, "Wrong check: Invalid Z")
}

func TestVerifyMisbehavingProof(t *testing.T) {
	clients, servers, context, _ := GenerateTestContext(2, 2)
	tagAndCommitments, s := newInitialTagAndCommitments(context.g.y, context.h[clients[0].index])

	//Generate a valid challenge
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}

	challenge, _ := InitializeChallenge(context, commits, openings)

	//Normal execution
	for _, server := range servers {
		server.CheckUpdateChallenge(context, challenge)
	}

	clientChallenge, _ := FinalizeChallenge(context, challenge)

	// setup test server "channels"
	pushCommitments, pullChallenge := newDummyServerChannels(clientChallenge)

	//Assemble the client message
	clientProof, err := newClientProof(*context, clients[0], *tagAndCommitments, s, pushCommitments, pullChallenge)
	assert.NoError(t, err, "failed to generate client proof, this is not expected")
	clientMessage := authenticationMessage{
		c:                        *context,
		initialTagAndCommitments: *tagAndCommitments,
		p0:                       clientProof,
	}

	proof, _ := servers[0].generateMisbehavingProof(context, clientMessage.sCommits[0])

	check := verifyMisbehavingProof(context, 0, proof, clientMessage.sCommits[0])
	assert.True(t, check, "Cannot verify valid misbehaving proof")

	//Invalid inputs
	check = verifyMisbehavingProof(nil, 0, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Wrong check: Invalid context")

	check = verifyMisbehavingProof(context, 1, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Wrong check: Invalid index")

	check = verifyMisbehavingProof(context, -1, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Wrong check: Negative index")

	check = verifyMisbehavingProof(context, 0, nil, clientMessage.sCommits[0])
	assert.False(t, check, "Wrong check: Missing proof")

	check = verifyMisbehavingProof(context, 0, proof, nil)
	assert.False(t, check, "Wrong check: Invalid Z")

	//Modify proof values
	proof, _ = servers[0].generateMisbehavingProof(context, clientMessage.sCommits[0])
	saveProof := serverProof{
		c:  proof.c,
		t1: proof.t1,
		t2: proof.t2,
		t3: proof.t3,
		r1: proof.r1,
		r2: proof.r2,
	}

	//Check inputs
	proof.c = nil
	check = verifyMisbehavingProof(context, 0, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Error in challenge verification")
	proof.c = saveProof.c

	proof.t1 = nil
	check = verifyMisbehavingProof(context, 0, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Error in t1 verification")
	proof.t1 = saveProof.t1

	proof.t2 = nil
	check = verifyMisbehavingProof(context, 0, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Error in t2 verification")
	proof.t2 = saveProof.t2

	proof.t3 = nil
	check = verifyMisbehavingProof(context, 0, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Error in t3 verification")
	proof.t3 = saveProof.t3

	proof.r1 = nil
	check = verifyMisbehavingProof(context, 0, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Error in r1 verification")
	proof.r1 = saveProof.r1

	proof.r2 = suite.Scalar().One()
	check = verifyMisbehavingProof(context, 0, proof, clientMessage.sCommits[0])
	assert.False(t, check, "Error in r2 verification")
	proof.r2 = saveProof.r2
	// TODO: Complete the tests
}

func TestGenerateNewRoundSecret(t *testing.T) {
	_, servers, _, _ := GenerateTestContext(1, 1)
	R, server := GenerateNewRoundSecret(servers[0])
	servers[0] = server
	assert.NotNil(t, R, "Cannot generate new round secret")
	assert.False(t, R.Equal(suite.Point().Mul(suite.Scalar().One(), nil)), "R is the generator")
	assert.NotNil(t, servers[0].r, "r was not saved to the server")
	assert.True(t, R.Equal(suite.Point().Mul(servers[0].r, nil)), "Mismatch between r and R")
}

func TestToBytes_ServerProof(t *testing.T) {
	clients, servers, context, _ := GenerateTestContext(2, 2)
	tagAndCommitments, s := newInitialTagAndCommitments(context.g.y, context.h[clients[0].index])
	_, S := tagAndCommitments.t0, tagAndCommitments.sCommits

	//Generate a valid challenge
	var commits []Commitment
	var openings []kyber.Scalar
	for i := 0; i < len(servers); i++ {
		commit, open, _ := servers[i].GenerateCommitment(context)
		commits = append(commits, *commit)
		openings = append(openings, open)
	}

	challenge, _ := InitializeChallenge(context, commits, openings)

	//Create challenge
	for _, server := range servers {
		server.CheckUpdateChallenge(context, challenge)
	}

	clientChallenge, _ := FinalizeChallenge(context, challenge)

	// setup test server "channels"
	pushCommitments, pullChallenge := newDummyServerChannels(clientChallenge)

	//Assemble the client message
	clientProof, err := newClientProof(*context, clients[0], *tagAndCommitments, s, pushCommitments, pullChallenge)
	assert.NoError(t, err, "failed to generate client proof, this is not expected")
	clientMessage := authenticationMessage{
		c:                        *context,
		initialTagAndCommitments: *tagAndCommitments,
		p0:                       clientProof,
	}

	servMsg := ServerMessage{request: clientMessage, proofs: nil, tags: nil, sigs: nil, indexes: nil}

	servers[0].ServerProtocol(context, &servMsg)

	//Normal execution for correct proof
	data, err := servMsg.proofs[0].ToBytes()
	assert.NoError(t, err, "Cannot convert normal proof")
	assert.NotNil(t, data, "Cannot convert normal proof")

	//Normal execution for correct misbehaving proof
	proof, _ := servers[0].generateMisbehavingProof(context, S[0])
	data, err = proof.ToBytes()
	assert.NoError(t, err, "Cannot convert misbehaving proof")
	assert.NotNil(t, data, "Cannot convert misbehaving proof")
}
