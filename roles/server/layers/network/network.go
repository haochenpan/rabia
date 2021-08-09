/*
    Copyright 2021 Rabia Research Team and Developers

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
*/
/*
	The network package defines the network layer of a server, which in charge of communicating with server peers. It
	sends messages produced by executors, messages received from network layer message handlers, and messages forwarded
	from the proxy to one or more servers (may include itself). When it receives a message from its peer, it routes the
	message to a channel (to a proxy, or an executor, or a handler) according to the message's type.

	Note: for messages of type ProposalRequest and ProposalReply, some fields besides the type fields are also used in
	determining routing destination. See the comment in msg.proto for more details.

	Comments on the sequence number / logical slot number / message sequence number:
	They mean the same thing and I use them interchangeably. Why "message sequence number" means the same is a little
	obscure, basically, messages except those of type ClientRequests has a slot number associated with it, and that
	number is called "message sequence number."
*/
package network

import (
	"fmt"
	. "rabia/internal/config"
	. "rabia/internal/message"
	"rabia/internal/tcp"
	"sync"
)

/*
	A Rabia server's peer networking layer (i.e., a network layer)
*/
type Network struct {
	SvrId uint32
	Wg    *sync.WaitGroup // for the network to mark it has exited
	Done  chan struct{}   // for the server to signal the network layer (and other layers) to exit

	ToProxy, ProxyIn            chan Msg
	MsgHandlerIn, ConExecutorIn chan Msg
	ToMsgHandler, ToConExecutor chan Msg
	ToSerializer                chan Msg

	TCP *tcp.NetTCP
}

/*
	Initiate the network layer
*/
func NetworkInit(svrId uint32, done chan struct{}, doneWg *sync.WaitGroup, netIp string,
	toProxy, proxyIn, msgHandlerIn, conExecutorIn chan Msg,
	toMsgHandler, toConExecutor chan Msg) *Network {
	n := &Network{
		SvrId: svrId,
		Wg:    doneWg,
		Done:  done,

		ToProxy:       toProxy,
		ProxyIn:       proxyIn,
		MsgHandlerIn:  msgHandlerIn,
		ConExecutorIn: conExecutorIn,
		ToMsgHandler:  toMsgHandler,
		ToConExecutor: toConExecutor,
		ToSerializer:  make(chan Msg, Conf.LenChannel),

		TCP: tcp.NetTCPInit(svrId, netIp),
	}
	return n
}

/*
	1. establish network-layer TCP connection(s)
*/
func (n *Network) Prologue() {
	n.TCP.Connect()
	fmt.Println("network = ", n.SvrId, "successfully connected to all servers")
}

/*
	1. close network-layer TCP connection(s)
*/
func (n *Network) Epilogue() {
	n.TCP.Close()
}

/*
	MsgRouter, the main routine of the network layer, see detailed comments below about how it routes messages
*/
func (n *Network) MsgRouter() {
	defer n.Wg.Done()
	defer close(n.ToSerializer)
MainLoop:
	for {
		select {
		case <-n.Done:
			break MainLoop

		case msg := <-n.ProxyIn: // ClientRequest msg
			n.ToSerializer <- msg

		case msg := <-n.MsgHandlerIn: // ProposalReply msg
			/*
				send to the peer which sends the respective ProposalRequest
				msg.Phase contains the destination server's id
			*/
			data, err := msg.Marshal() // gogo-protobuf
			//data, err := proto.Marshal(&msg) // vanilla protobuf
			if err != nil {
				panic(fmt.Sprint("should not happen, marshal error", err))
			}
			n.TCP.SendChan[msg.Phase] <- data

		case msg := <-n.ConExecutorIn: // Proposal, State, Vote, ProposalRequest, and Decision msg
			/*
				broadcast the message so all peers' network layers can receive it
			*/
			n.ToSerializer <- msg

		/*
			receives a message from a peer
		*/
		case msg := <-n.TCP.RecvChan:
			if msg.Type == ProposalReply {
				n.ToConExecutor <- msg // sends the proposal reply to the executor directly
			} else {
				n.ToMsgHandler <- msg
			}
		}
	}
}

/*
	MsgSerializer routine serialize messages of type Msg to byte arrays. So that multiple NetworkTCP SendHandlers
	do not need to serialize the same message repeatedly, instead, they take the serialized byte arrays and send
	them to different peers through TCP connections. For each msg in ToSerializer, MsgSerializer serializes it and
	send it to all send channels.
*/
func (n *Network) MsgSerializer() {
	defer n.Wg.Done()
	for msg := range n.ToSerializer {
		data, err := msg.Marshal() // gogo-protobuf
		//data, err := proto.Marshal(&msg) // vanilla protobuf
		if err != nil {
			panic(fmt.Sprint("should not happen, marshal error", err))
		}
		for _, t := range n.TCP.SendChan { // broadcasting
			t <- data
		}
	}
}
