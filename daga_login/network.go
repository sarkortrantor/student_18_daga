package daga_login

import (
	"errors"
	"fmt"
	"github.com/dedis/kyber"
	"github.com/dedis/student_18_daga/sign/daga"
)

// contain net wrappers around the kyber.daga datastructures and

// TODO QUESTION ask, dumb IMO but feel kind of bad exporting things that are intended to be immutable and private so the in between solution is to have a separate struct
// TODO ~messy IMO, how to do it in a idiomatic and educated way ?

// to represent a daga.Client (which is an interface)
// used only to dump client to disk while developing for now
type NetClient struct {
	PrivateKey kyber.Scalar
	Index      int
}

// to represent a daga.Server (which is an interface)
// used only to dump server to disk while developing for now
type NetServer struct {
	PrivateKey     kyber.Scalar
	Index          int
	PerRoundSecret kyber.Scalar
}

// NetAuthenticationMessage provides a net compatible representation of the daga.AuthenticationMessage struct
// (which embeds a context which is an interface)
type NetAuthenticationMessage struct {
	Context  Context
	SCommits []kyber.Point
	T0       kyber.Point
	Proof    daga.ClientProof
}

// NetServerMessage provides a net compatible representation of the daga.ServerMessage struct
// (which embeds an auth message struct which embeds a context which ..)
type NetServerMessage struct {
	Request NetAuthenticationMessage
	Tags    []kyber.Point
	Proofs  []daga.ServerProof
	Indexes []int
	Sigs    []daga.ServerSignature
}

func NetEncodeClient(c daga.Client) *NetClient {
	return &NetClient{
		Index:      c.Index(),
		PrivateKey: c.PrivateKey(),
	}
}

func (nc NetClient) NetDecode() (*Client, error) {
	return NewClient(nc.Index, nc.PrivateKey)
}

func NetEncodeClients(clients []daga.Client) ([]NetClient, error) {
	if len(clients) == 0 {
		return nil, errors.New("empty array")
	}
	var netClients []NetClient
	for _, client := range clients {
		netClient := NetEncodeClient(client)
		netClients = append(netClients, *netClient)
	}
	return netClients, nil
}

func NetDecodeClients(netClients []NetClient) ([]*Client, error) {
	if len(netClients) == 0 {
		return nil, errors.New("empty slice")
	}
	var clients []*Client
	for i, nc := range netClients {
		if c, err := nc.NetDecode(); err != nil {
			return nil, fmt.Errorf("Decode error at index %d\n%s", i, err)
		} else {
			clients = append(clients, c)
		}
	}
	return clients, nil
}

func NetEncodeServer(s daga.Server) *NetServer {
	return &NetServer{
		PrivateKey:     s.PrivateKey(),
		Index:          s.Index(),
		PerRoundSecret: s.RoundSecret(),
	}
}

func NetEncodeServers(servers []daga.Server) ([]NetServer, error) {
	if len(servers) == 0 {
		return nil, errors.New("empty array")
	}
	var netServers []NetServer
	for _, server := range servers {
		netServer := NetEncodeServer(server)
		netServers = append(netServers, *netServer)
	}
	return netServers, nil
}

func (s NetServer) NetDecode() (daga.Server, error) {
	server, err := daga.NewServer(suite, s.Index, s.PrivateKey)
	if err != nil {
		return nil, err
	}
	server.SetRoundSecret(s.PerRoundSecret)
	return server, nil
}

func NetEncodeAuthenticationMessage(context Context, msg daga.AuthenticationMessage) *NetAuthenticationMessage {
	return &NetAuthenticationMessage{
		Context:  context, // i.e. discard context part of message and use the one provided
		T0:       msg.T0,
		SCommits: msg.SCommits,
		Proof:    msg.P0,
	}
}

func (netmsg NetAuthenticationMessage) NetDecode() (*daga.AuthenticationMessage, Context, error) {
	msg := daga.AuthenticationMessage{
		C:  netmsg.Context.MinimumAuthenticationContext,
		P0: netmsg.Proof,
	}
	msg.SCommits = netmsg.SCommits
	msg.T0 = netmsg.T0
	return &msg, netmsg.Context, nil
}

func NetEncodeServerMessage(context Context, msg *daga.ServerMessage) *NetServerMessage {
	request := NetEncodeAuthenticationMessage(context, msg.Request)
	return &NetServerMessage{
		Request: *request,
		Sigs:    msg.Sigs,
		Proofs:  msg.Proofs,
		Tags:    msg.Tags,
		Indexes: msg.Indexes,
	}
}

func (netmsg NetServerMessage) NetDecode() (*daga.ServerMessage, Context, error) {
	request, context, err := netmsg.Request.NetDecode()
	if err != nil {
		return nil, Context{}, fmt.Errorf("failed to decode request: %s", err)
	}
	return &daga.ServerMessage{
		Request: *request,
		Tags:    netmsg.Tags,
		Proofs:  netmsg.Proofs,
		Sigs:    netmsg.Sigs,
		Indexes: netmsg.Indexes,
	}, context, nil
}
