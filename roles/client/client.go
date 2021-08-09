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
	The client package defines the struct and functions of a Rabia client. There are two types of clients, open-loop
	ones and closed-loop ones. The former sends requests one after another until they send Conf.NClientRequests
	requests, and waits are requests are replied. The latter waits for a reply after sending a request until the
	benchmark time ends, i.e., when Conf.ClientTimeout is reached. Each request contains one more key-value store
	operations.

	Note:

	1. The purpose of ClientBatchSize:

	When ClientBatchSize = 1, each Client routine can be conceived as a KV store client, i.e., a kv-store client
	process that sends one request and receives one reply at a time.

	When ClientBatchSize = n >= 1, each Client routine can be conceived as a client proxy, which batches one or more
	kv-store client processes' requests, and sends a batch of requests and receives a batch of requests at a time.

	See comments in msg.proto for more discussion.
*/
package client

import (
	"fmt"
	"github.com/rs/zerolog"
	"math"
	"math/rand"
	"os"
	. "rabia/internal/config"
	"rabia/internal/logger"
	. "rabia/internal/message"
	"rabia/internal/rstring"
	"rabia/internal/system"
	"rabia/internal/tcp"
	"sort"
	"sync"
	"time"
)

/*
	A clients sends one or more requests (i.e., DB read or write operations) at a time, we note down the send time and
	receive time in the following data structure
*/
type BatchedCmdLog struct {
	SendTime    time.Time     // the send time of this client-batched command
	ReceiveTime time.Time     // the receive time of client-batched command
	Duration    time.Duration // the calculate latency of this command (ReceiveTime - SendTime)
}

/*
	A Rabia client
*/
type Client struct {
	ClientId uint32
	Wg       *sync.WaitGroup // wait any subroutines
	Done     chan struct{}

	TCP     *tcp.ClientTCP
	Rand    *rand.Rand
	Logger  zerolog.Logger // the real-time server log that help to track throughput and the number of connections
	LogFile *os.File       // the log file that should be called .Sync() method before the routine exits

	CommandLog                             []BatchedCmdLog
	SentSoFar, ReceivedSoFar               int
	startSending, endSending, endReceiving time.Time
}

/*
	Initialize a Rabia client
*/
func ClientInit(clientId uint32, proxyIp string) *Client {
	zerologger, logFile := logger.InitLogger("client", clientId, 0, "both")
	c := &Client{
		ClientId: clientId,
		Wg:       &sync.WaitGroup{},
		Done:     make(chan struct{}),

		TCP:     tcp.ClientTcpInit(clientId, proxyIp),
		Rand:    rand.New(rand.NewSource(time.Now().UnixNano() * int64(clientId))),
		Logger:  zerologger,
		LogFile: logFile,

		CommandLog: make([]BatchedCmdLog, Conf.NClientRequests/Conf.ClientBatchSize),
	}
	/*
		SentSoFar, ReceivedSoFar are zeros are initialization
		startSending, endSending, endReceiving retain their default values
	*/
	return c
}

/*
	1. start the OS signal listener
	2. establish the TCP connection with a designated proxy
	3. starts a terminal logger
*/
func (c *Client) Prologue() {
	go system.SigListen(c.Done) //
	c.TCP.Connect()
	go c.terminalLogger()
}

/*
	1. close the Done channel to inform other routines who listen to this signal to exit
	2. write a concluding log to file
	3. close the log file
	4. close the TCP connection
*/
func (c *Client) Epilogue() {
	close(c.Done)
	c.writeToLog()
	if err := c.LogFile.Sync(); err != nil {
		panic(err)
	}
	c.TCP.Close()
}

/*
	The main body of a closed-loop client.
	A closed-loop client sends one (batched) request and waits for a reply at a time. In waiting for a reply, if the
	client finds the Conf.ClientTimeout time is reached, it exits the loop.

*/
func (c *Client) CloseLoopClient() {
	c.startSending = time.Now()
	ticker := time.NewTicker(Conf.ClientTimeout)
MainLoop:
	for i := 0; i < Conf.NClientRequests/Conf.ClientBatchSize; i++ {
		c.sendOneRequest(i)
		select {
		case rep := <-c.TCP.RecvChan:
			c.processOneReply(rep)
		case <-ticker.C:
			break MainLoop
		}
	}
	c.endSending = time.Now()
	c.endReceiving = time.Now()
}

/*
	The main body of a open-loop client.
*/
func (c *Client) OpenLoopClient() {
	c.Wg.Add(2)
	go func() {
		c.startSending = time.Now()
		for i := 0; i < Conf.NClientRequests/Conf.ClientBatchSize; i++ {
			if c.SentSoFar-c.ReceivedSoFar >= 10000*Conf.ClientBatchSize {
				time.Sleep(500 * time.Millisecond)
				i--
				continue
			}
			c.sendOneRequest(i)
		}
		fmt.Println(c.ClientId, "client requests all sent")
		c.endSending = time.Now()
		c.Wg.Done()
	}()
	go func() {
		for i := 0; i < Conf.NClientRequests/Conf.ClientBatchSize; i++ {
			rep := <-c.TCP.RecvChan
			c.processOneReply(rep)
		}
		c.endReceiving = time.Now()
		c.Wg.Done()
	}()
	c.Wg.Wait()
}

/*
	Sends a single request.
	val is a string of 17 bytes (modifiable through Conf.KeyLen and Conf.ValLen)
	[0:1]   (1 byte): "0" == a write operation,  "1" == a read operation
	[1:9]  (8 bytes): a string Key
	[9:17] (8 bytes): a string Value
*/
func (c *Client) sendOneRequest(i int) {
	obj := Command{CliId: c.ClientId, CliSeq: uint32(i), Commands: make([]string, Conf.ClientBatchSize)}
	for j := 0; j < Conf.ClientBatchSize; j++ {
		val := fmt.Sprintf("%d%v%v", c.Rand.Intn(2),
			rstring.RandString(c.Rand, Conf.KeyLen),
			rstring.RandString(c.Rand, Conf.ValLen))
		obj.Commands[j] = val
	}

	time.Sleep(time.Duration(Conf.ClientThinkTime) * time.Millisecond)

	c.CommandLog[i].SendTime = time.Now()
	c.TCP.SendChan <- obj
	c.SentSoFar += Conf.ClientBatchSize
}

/*
	Processes on received reply
*/
func (c *Client) processOneReply(rep Command) {
	if c.CommandLog[rep.CliSeq].Duration != time.Duration(0) {
		panic("already received")
	}
	c.CommandLog[rep.CliSeq].ReceiveTime = time.Now()
	c.CommandLog[rep.CliSeq].Duration = c.CommandLog[rep.CliSeq].ReceiveTime.Sub(c.CommandLog[rep.CliSeq].SendTime)
	c.ReceivedSoFar += Conf.ClientBatchSize
}

/*
	A terminal logger that prints the status of a client to terminal
*/
func (c *Client) terminalLogger() {
	tLogger, file := logger.InitLogger("client", c.ClientId, 1, "both")
	defer func() {
		if err := file.Sync(); err != nil {
			panic(err)
		}
	}()
	ticker := time.NewTicker(Conf.ClientLogInterval)

	lastRecv, thisRecv := 0, 0
	for {
		select {
		case <-c.Done:
			return
		case <-ticker.C:
			thisRecv = c.ReceivedSoFar
			tho := math.Round(float64(thisRecv-lastRecv) / Conf.ClientLogInterval.Seconds())
			tLogger.Warn().
				Uint32("Client Id", c.ClientId).
				Int("Sent", c.SentSoFar).
				Int("Recv", thisRecv).
				Float64("Interval Recv Tput (cmd/sec)", tho).Msg("")
			lastRecv = thisRecv
		}
	}
}

/*
	Note: logs produced from there are for eye-inspection, they are often baised in telling how the system performs.
	Maybe consider using the log produced from the system-end to calculate throughput and latencies.
*/
func (c *Client) writeToLog() {
	RepliedLength := len(c.CommandLog) // assume all replied
	for i := 0; i < len(c.CommandLog); i++ {
		if c.CommandLog[i].Duration == time.Duration(0) {
			//c.Logger.Warn("", Int("not replied", i))
			RepliedLength = i // update RepliedLength if necessary
			break
		}
	}
	//fmt.Println("RepliedLength =", RepliedLength)

	// cmdLogs -- exclude head and tails statistics in BatchedCmdLog:
	cmdLogs := make([]BatchedCmdLog, int(float64(RepliedLength)*0.8))
	j := 0
	for i := 0; i < len(c.CommandLog); i++ {
		if i < int(float64(RepliedLength)*0.1) ||
			i >= int(float64(RepliedLength)*0.9) {
			continue
		}
		if j < len(cmdLogs) {
			cmdLogs[j] = c.CommandLog[i]
			j++
		} else {
			break
		}
	}
	//fmt.Println("RepliedLength =", RepliedLength,
	//	"len of cmdLogs = ", len(cmdLogs), ",", j, "items filled")

	maxLatVal := time.Duration(0)
	maxLatIdx := 0
	for i, cmd := range cmdLogs {
		if cmd.Duration > maxLatVal {
			maxLatVal = cmd.Duration
			maxLatIdx = i + int(float64(RepliedLength)*0.1)
		}
	}
	mid80Start := cmdLogs[0].SendTime
	mid80End := cmdLogs[len(cmdLogs)-1].ReceiveTime
	mid80Dur := mid80End.Sub(mid80Start).Seconds()

	mid80FirstRecvTime := cmdLogs[0].ReceiveTime
	mid80RecvTimeDur := mid80End.Sub(mid80FirstRecvTime).Seconds()

	sort.Slice(cmdLogs, func(i, j int) bool {
		return cmdLogs[i].Duration < cmdLogs[j].Duration
	})
	minLat := cmdLogs[0].Duration
	maxLat := cmdLogs[len(cmdLogs)-1].Duration
	p50Lat := cmdLogs[int(float64(len(cmdLogs))*0.5)].Duration
	p95Lat := cmdLogs[int(float64(len(cmdLogs))*0.9)].Duration
	p99Lat := cmdLogs[int(float64(len(cmdLogs))*0.99)].Duration
	var durSum int64
	for _, v := range cmdLogs {
		durSum += v.Duration.Microseconds()
	}
	durAvg := durSum / int64(len(cmdLogs))

	c.Logger.Warn().
		Uint32("ClientId", c.ClientId).
		Int("TotalSent", c.SentSoFar).
		Int("TotalRecv", c.ReceivedSoFar).
		Int64("minLat", minLat.Microseconds()).
		Int64("maxLat", maxLat.Microseconds()).
		Int("maxLatIdx", maxLatIdx).
		Int64("avgLat", durAvg).
		Int64("p50Lat", p50Lat.Microseconds()).
		Int64("p95Lat", p95Lat.Microseconds()).
		Int64("p99Lat", p99Lat.Microseconds()).
		Int64("sendStart", c.startSending.UnixNano()).
		Int64("sendEnd", c.endSending.UnixNano()).
		Int64("recvEnd", c.endReceiving.UnixNano()).
		Int64("mid80Start", mid80Start.UnixNano()).
		Int64("mid80End", mid80End.UnixNano()).
		Float64("mid80Dur", mid80Dur).
		Float64("mid80RecvTimeDur", mid80RecvTimeDur).
		Int("mid80Requests", len(cmdLogs)).
		Float64("mid80Throughput (cmd/sec)", float64(len(cmdLogs))/mid80Dur).
		Float64("mid80Throughput2 (cmd/sec)", float64(len(cmdLogs))/mid80RecvTimeDur).Msg("")
}
