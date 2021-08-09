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
	The server package defines a Rabia server's struct and its initialization, preparation, and main function. Note: A
	server has many Goroutines, and each routine belongs to one of the three server layers: the proxy/application layer,
	the peer-communicating network layer, and the consensus instance layer. There are one or two main Go routines at
	each layer. When a server wants to exit, it needs to wait for them all. Besides, the proxy and network layers
	maintain their TCP connections, which require a few more Goroutines to send requests and to wait for responses.
*/
package server

import (
	"fmt"
	"github.com/rs/zerolog"
	"math"
	"os"
	. "rabia/internal/config"
	"rabia/internal/ledger"
	"rabia/internal/logger"
	. "rabia/internal/message"
	"rabia/internal/system"
	"rabia/roles/server/layers/consensus"
	"rabia/roles/server/layers/network"
	"rabia/roles/server/layers/proxy"
	"sync"
	"time"
)

/*
	A Rabia server
*/
type Server struct {
	SvrId uint32
	Wg    *sync.WaitGroup
	Done  chan struct{}

	Ledger  []*ledger.Slot
	Logger  zerolog.Logger // the real-time server log that help to track throughput and the number of connections
	LogFile *os.File       // the log file that should be called .Sync() method before the routine exits,
	// see the last a few lines of Executor.Executor() function for an example

	Proxy     *proxy.Proxy
	Network   *network.Network
	Consensus *consensus.Consensus

	ClientsToProxy                    chan Command
	ProxyToNet, NetToProxy            chan Msg
	MsgHandlerToNet, ConExecutorToNet chan Msg
	NetToMsgHandler, NetToConExecutor chan Msg
}

/*
	Initialize a server -- go channels and routine allocations

	Difference between *Init() functions and *Prepare/Prologue()  functions in the Rabia project:
	*Init(): allocating objects, does not involve the network activity (except allocating a listener)
	*Prepare/Prologue(): involves the network activity and starts to listen to OS signals
*/
func ServerInit(svrId uint32, proxyIp, netIp string) *Server {
	s := &Server{
		SvrId: svrId,
		Wg:    &sync.WaitGroup{},
		Done:  make(chan struct{}),

		Ledger: make(ledger.Ledger, Conf.LenLedger),

		ClientsToProxy:   make(chan Command),
		ProxyToNet:       make(chan Msg, Conf.LenChannel),
		NetToProxy:       make(chan Msg, Conf.LenChannel),
		MsgHandlerToNet:  make(chan Msg, Conf.LenChannel),
		ConExecutorToNet: make(chan Msg, Conf.LenChannel),
		NetToMsgHandler:  make(chan Msg, Conf.LenChannel),
		NetToConExecutor: make(chan Msg, Conf.LenChannel),
	}
	// s.Logger, s.LogFile =  logger.InitLogger("server", svrId, 0, "both")
	// note: the log file of this logger should be synced! e.g., see the last a few lines of Executor.Executor()

	s.NetToMsgHandler = make(chan Msg, Conf.LenChannel)
	s.NetToConExecutor = make(chan Msg, Conf.LenChannel)
	for i := 0; i < int(Conf.LenLedger); i++ {
		s.Ledger[i] = &ledger.Slot{}
		s.Ledger[i].Reset()
	}
	s.Proxy = proxy.ProxyInit(svrId, s.Done, s.Wg, proxyIp, s.ClientsToProxy, s.ProxyToNet, s.NetToProxy, s.Ledger)
	s.Network = network.NetworkInit(svrId, s.Done, s.Wg, netIp, s.NetToProxy, s.ProxyToNet, s.MsgHandlerToNet,
		s.ConExecutorToNet, s.NetToMsgHandler, s.NetToConExecutor)
	s.Consensus = consensus.ConsensusInit(svrId, 0, s.Done, s.Wg, s.NetToMsgHandler,
		s.MsgHandlerToNet, s.NetToConExecutor, s.ConExecutorToNet, s.Ledger)
	return s
}

/*
	1. start the OS signal listener
	2. add the number of major routines in Proxy and Network layers to Wg
	3. start the network layer
	4. start the proxy layer
	5. starts a terminal logger
*/
func (s *Server) Prologue() {
	go system.SigListen(s.Done)
	s.Network.Prologue()
	s.Proxy.Prologue()
	go s.TerminalLogger()
}

/*
	1. start the proxy layer (two separate routines)
	2. start the network layer (two separate routines)
	3. start the consensus layer (two separate routines)
	4. wait all layers to finish
*/
func (s *Server) ServerMain() {
	s.Wg.Add(2)
	go s.Proxy.CmdReceiver()
	go s.Proxy.KVSExecutor()

	s.Wg.Add(2)
	go s.Network.MsgRouter()
	go s.Network.MsgSerializer()

	s.Wg.Add(2)
	go s.Consensus.Executor()
	go s.Consensus.MsgHandler()

	<-s.Done
}

/*
	1. calling proxy level exit
	2. calling network level exit
	3. wait major routines are done
*/
func (s *Server) Epilogue() {
	s.Proxy.Epilogue()
	s.Network.Epilogue()
	s.Wg.Wait()
}

/*
	A terminal logger that prints the status of a server to terminal
*/
func (s *Server) TerminalLogger() {
	tLogger, file := logger.InitLogger("server", s.SvrId, 1, "both")
	defer func() {
		if err := file.Sync(); err != nil {
			panic(fmt.Sprint("error syncing file", err))
		}
	}()
	ticker := time.NewTicker(Conf.SvrLogInterval)

	lastNotNulls := 0
	lastCBProcessed := 0
	for {
		select {
		case <-s.Done:
			return
		case <-ticker.C:
			var proxyConnect int
			for _, c := range s.Proxy.TCP.Conns {
				if c != nil {
					proxyConnect++
				}
			}

			thisNotNulls := s.Consensus.NormalSlots + s.Consensus.UnmatchedSlots
			thisCBProcessed := s.Consensus.NumClientBatchedRequests
			throughput := math.Round(float64((thisCBProcessed-lastCBProcessed)*Conf.ClientBatchSize) / Conf.SvrLogInterval.Seconds())
			// items below may not appear in this order, see https://github.com/rs/zerolog/issues/50
			tLogger.Warn().
				Uint32("Svr Id", s.SvrId).
				Int("Client Conn.", proxyConnect).
				Int("Normal Slots", s.Consensus.NormalSlots).
				Int("Unmatched Slots", s.Consensus.UnmatchedSlots).
				Int("NULL Slots", s.Consensus.NullSlots).
				Int("Interval not-NULL Slots", thisNotNulls-lastNotNulls).
				Float64("Interval throughput (cmd/sec)", throughput).Msg("")
			lastNotNulls = thisNotNulls
			lastCBProcessed = thisCBProcessed
		}
	}
}
