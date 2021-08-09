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
	The tcp package defines ClientTCP, ProxyTCP, and NetTCP objects, which are the TCP communication component of a
	client, a server's proxy layer, and a server network layer, respectively. These objects and their functions help
	establish and maintain TCP connections and let Go routines to listen to SendChan (the channel that queues messages
	to be sent) and forward messages received over TCP to RecvChan (to be subsequently accessed by caller routines).

	Note:

	1. This package assumes Conf is initialized.

	2. This version of TCP endpoints does no support reconfiguration. Reconfiguring a server requires having multiple
	TCP endpoint objects
*/
package tcp

import (
	"bufio"
	"fmt"
	"net"
	. "rabia/internal/config"
	. "rabia/internal/message"
	"sync"
	"time"
)

/*
	Generates a reader and a writer from a connection.

	Note: I suspect that if we call this function twice, the newly generated reader and writer will replace the
	previously allocated reader and writer. Be aware of any side-effects.
*/
func GetReaderWriter(conn *net.Conn) (*bufio.Reader, *bufio.Writer) {
	var err error
	err = (*conn).(*net.TCPConn).SetWriteBuffer(Conf.TcpBufSize)
	if err != nil {
		panic(fmt.Sprint("should not happen", err))
	}
	err = (*conn).(*net.TCPConn).SetReadBuffer(Conf.TcpBufSize)
	if err != nil {
		panic(fmt.Sprint("should not happen", err))
	}
	err = (*conn).(*net.TCPConn).SetKeepAlive(true)
	if err != nil {
		panic(fmt.Sprint("should not happen", err))
	}
	err = (*conn).(*net.TCPConn).SetKeepAlivePeriod(20 * time.Second)
	if err != nil {
		panic(fmt.Sprint("should not happen", err))
	}
	reader := bufio.NewReaderSize(*conn, Conf.IoBufSize)
	writer := bufio.NewWriterSize(*conn, Conf.IoBufSize)
	return reader, writer
}

/*
	Client TCP, each client connects to a single proxy
*/
type ClientTCP struct {
	Id   uint32          // client id
	Wg   *sync.WaitGroup // waits SendHandler and RecvHandler
	Done chan struct{}   // waits SendHandler and RecvHandler

	ProxyAddr string       // the address of a proxy that the client intends to connect to
	RecvChan  chan Command // RecvHandler receives Command objects from Reader and then sends to this channel
	SendChan  chan Command // SendHandler receives Command objects from this channel and sends to Writer

	Conn   *net.Conn     // the connection to a proxy
	Reader *bufio.Reader // the reader that binds to the Conn object
	Writer *bufio.Writer // the writer that binds to the Conn object
}

/*
	Returns a ClientTCP with channels initialized, but not Conns, Reader, and Writer fields
*/
func ClientTcpInit(Id uint32, ProxyIp string) *ClientTCP {
	c := &ClientTCP{
		Id:   Id,
		Wg:   &sync.WaitGroup{},
		Done: make(chan struct{}),

		ProxyAddr: ProxyIp,
		RecvChan:  make(chan Command, Conf.LenChannel),
		SendChan:  make(chan Command, Conf.LenChannel),
	}
	return c
}

func (c *ClientTCP) connect() {
	for {
		conn, err := net.Dial("tcp", c.ProxyAddr)
		if err == nil {
			c.Conn = &conn
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	c.Reader, c.Writer = GetReaderWriter(c.Conn)

	cmd := &Command{CliId: c.Id}
	err := cmd.MarshalWriteFlush(c.Writer)
	if err != nil {
		panic(err)
	}
	c.Wg.Add(2)
	go c.RecvHandler()
	go c.SendHandler()
}

func (c *ClientTCP) Connect() {
	go c.connect()
}

/*
	For each message received, send the message to RecvChan.
	RecvHandler exits when the channel is closed.
*/
func (c *ClientTCP) RecvHandler() {
	defer c.Wg.Done()
	readBuf := make([]byte, 4096*100)
	for {
		var cmd Command
		err := cmd.ReadUnmarshal(c.Reader, readBuf)
		if err != nil { // TCP connection closed
			return
		}
		c.RecvChan <- cmd
	}
}

/*
	For each message in SendChan, marshal the message and flush its bytes to the TCP connection through Writer before
	it exits (when c.Done channel is closed, SendHandler exits).
	Note:

	1. it is likely that c.Done closes before SendHandler sends every message ever passed in SendChan. In that case,
	some messages are remained in the channel but not sent.

	2. if the receiver has closed its connection, it is likely that no error msg is produced here at the sender; if the
	sender has closed its connection, error indeed happens.
*/
func (c *ClientTCP) SendHandler() {
	defer c.Wg.Done()
	for {
		select {
		case <-c.Done:
			return
		case req := <-c.SendChan:
			err := req.MarshalWriteFlush(c.Writer)
			if err != nil {
				panic(err)
			}
		}
	}
}

/*
	Prints the connection status.
*/
func (c *ClientTCP) PrintStatus() {
	fmt.Printf("ClientTcp, SvrId=%d, ProxyAddr=%s, Conns.local=%s, Conns.remote=%s\n",
		c.Id, c.ProxyAddr, (*c.Conn).LocalAddr(), (*c.Conn).RemoteAddr())
}

/*
	Closes the connection and waits SendHandler and RecvHandler to exit.
*/
func (c *ClientTCP) Close() {
	_ = (*c.Conn).Close()
	close(c.Done)
	c.Wg.Wait()
}

/*
	Proxy TCP, each proxy connects to one or more clients
*/
type ProxyTCP struct {
	Id   uint32          // proxy id
	Wg   *sync.WaitGroup // waits SendHandlers and RecvHandlers
	Done chan struct{}   // waits SendHandlers and RecvHandlers

	ProxyAddr string
	RecvChan  chan Command   // from clients to the proxy
	SendChan  []chan Command // from the proxy to clients

	Listener net.Listener
	Conns    []*net.Conn
	Readers  []*bufio.Reader
	Writers  []*bufio.Writer
}

// Allocates the ProxyTCP object without accepting connections from its clients
func ProxyTcpInit(Id uint32, ProxyIp string, ToProxy chan Command) *ProxyTCP {
	listener, err := net.Listen("tcp", ProxyIp)
	if err != nil {
		panic(err)
	}

	p := &ProxyTCP{
		Id:   Id,
		Wg:   &sync.WaitGroup{},
		Done: make(chan struct{}),

		ProxyAddr: ProxyIp,
		RecvChan:  ToProxy,
		SendChan:  make([]chan Command, Conf.NClients), // at most NClients clients can connect to this proxy

		Listener: listener,
		Conns:    make([]*net.Conn, Conf.NClients),
		Readers:  make([]*bufio.Reader, Conf.NClients),
		Writers:  make([]*bufio.Writer, Conf.NClients),
	}
	/*
		Note: SendChan, Conns, Readers, and Writers entries are not initialized at this points.
		Why arrays are of length NClients but not Clients[id]?
		Because the proxy needs to perform quick lookups of clients, see "if p.TCP.Conns[cid] != nil" on proxy.go
	*/
	return p
}

// Waits all its clients to connect
func (p *ProxyTCP) connect() {
	// Conf.NClients is an upper bound, but in common cases,
	// a proxy is connected to (Conf.NClients / Conf.NServers) clients
	for i := 0; i < Conf.NClients; i++ {
		conn, err := p.Listener.Accept()
		if err != nil {
			//fmt.Printf("ProxyTCP%d: connection accept thread exits\n", Conf.SvrId)
			return
		}

		reader, writer := GetReaderWriter(&conn)
		var req Command
		readBuf := make([]byte, 20)
		err = req.ReadUnmarshal(reader, readBuf)
		if err != nil {
			panic(err)
		}

		CliId := req.CliId
		p.Conns[CliId] = &conn
		p.SendChan[CliId] = make(chan Command, Conf.LenChannel)
		p.Writers[CliId] = writer
		p.Readers[CliId] = reader
		p.Wg.Add(2)
		go p.SendHandler(int(CliId))
		go p.RecvHandler(int(CliId))
	}
}

/*
	Proxy's Connect function is asynchronous - it exits early before connection to all clients (but it has a background
	routine that waits connections). The primary purpose of this function is exiting correctly. If we want the proxy to
	exit before connecting to all clients (due to some error or misconfiguration may happened), this function needs to
	be non-blocking ("Listener.Accept()" is blocking).
*/
func (p *ProxyTCP) Connect() {
	go func() { p.connect() }()
}

func (p *ProxyTCP) RecvHandler(from int) {
	defer p.Wg.Done()
	readBuf := make([]byte, 4096*100)
	for {
		var c Command
		err := c.ReadUnmarshal(p.Readers[from], readBuf)
		if err != nil {
			// maybe: TCP connection is closed or receives an ill-formed message
			return
		}
		p.RecvChan <- c
	}
}

func (p *ProxyTCP) SendHandler(to int) {
	defer p.Wg.Done()
	for {
		select {
		case <-p.Done:
			return
		case c := <-p.SendChan[to]:
			err := c.MarshalWriteFlush(p.Writers[to])
			if err != nil {
				return
			}
		}
	}
}

func (p *ProxyTCP) PrintStatus() {
	fmt.Printf("proxyTcp, SvrId=%d, ProxyAddr=%s\n", p.Id, p.ProxyAddr)
	for i := 0; i < Conf.NClients; i++ {
		if p.Conns[i] != nil {
			fmt.Printf("\t client id=%d, ip=%s\n", i, (*p.Conns[i]).RemoteAddr())
		}
	}
}

func (p *ProxyTCP) Close() {
	for i := 0; i < Conf.NClients; i++ {
		if p.Conns[i] != nil {
			_ = (*p.Conns[i]).Close()
		}
	}
	close(p.Done)
	_ = p.Listener.Close()
	p.Wg.Wait()
}

/*
	Network Layer TCP endpoints, each Rabia server has exactly one NetTCP struct and one network address (NetAddr
	below). Each server needs to dial to all peers (including itself) to establish send TCP channels, and needs to
	accept connections from all peers (and itself) to establish receive TCP channels.

	For example, when N = 3, each server does the following in Connect initially:
		waits server 0-2 to connect
		connects to server 0-2

	For example, when N = 5, each server does the following in Connect initially:
		waits server 0-4 to connect
		connects to server 0-4
*/
type NetTCP struct {
	Id   uint32
	Done chan struct{}
	Wg   *sync.WaitGroup

	NetAddr  string
	RecvChan chan Msg
	SendChan []chan []byte
	/*
		Note: each SendChan is of type "chan []byte" but not "chan Command" because for each message to be broadcasted,
		we only need to serialize once and let each SendHandler sends the serialized array of bytes.
	*/

	Listener net.Listener
	RecvConn []*net.Conn
	SendConn []*net.Conn
	Readers  []*bufio.Reader
	Writers  []*bufio.Writer
}

func NetTCPInit(Id uint32, NetIp string) *NetTCP {
	listener, err := net.Listen("tcp", NetIp)
	if err != nil {
		panic(err)
	}
	if len(Conf.Peers) != Conf.NServers {
		panic(fmt.Sprint("should not happen", err))
	}

	n := &NetTCP{
		Id:   Id,
		Wg:   &sync.WaitGroup{},
		Done: make(chan struct{}),

		NetAddr:  NetIp,
		RecvChan: make(chan Msg, Conf.LenChannel),
		SendChan: make([]chan []byte, Conf.NServers),

		Listener: listener,
		RecvConn: make([]*net.Conn, Conf.NServers),
		SendConn: make([]*net.Conn, Conf.NServers),
		Readers:  make([]*bufio.Reader, Conf.NServers),
		Writers:  make([]*bufio.Writer, Conf.NServers),
	}

	/*
		Note: SendChan, RecvConn, SendConn, Readers, Writers entries are not initialized at this points.
	*/
	return n
}

// Listens to all peers (and itself), fill in the respective entries in RecvConn and Readers.
func (n *NetTCP) accepting() {
	for i := 0; i < Conf.NServers; i++ {
		conn, err := n.Listener.Accept()
		if err != nil {
			panic(err)
		}

		reader, _ := GetReaderWriter(&conn)
		var c Command
		readBuf := make([]byte, 20)
		err = c.ReadUnmarshal(reader, readBuf)
		if err != nil {
			panic(err)
		}

		id := c.CliId
		n.RecvConn[id] = &conn
		n.Readers[id] = reader
	}
}

// Dials to all peers (and itself), fill in the respective entries in SendChan, SendConn, and Writers.
func (n *NetTCP) dialing() {
	if len(Conf.Peers) != Conf.NServers {
		panic(fmt.Sprint("should not happen, len(Conf.Peers) != Conf.NServers"))
	}
	for i := 0; i < Conf.NServers; i++ {
		var conn net.Conn
		var err error
		conn, err = net.Dial("tcp", Conf.Peers[i])
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			i--
			continue
		}

		n.SendChan[i] = make(chan []byte, Conf.LenChannel)
		n.SendConn[i] = &conn

		_, n.Writers[i] = GetReaderWriter(&conn)

		c := &Command{CliId: n.Id}
		err = c.MarshalWriteFlush(n.Writers[i])
		if err != nil {
			panic(err)
		}
	}
}

func (n *NetTCP) Connect() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		n.accepting()
		wg.Done()
	}()
	go func() {
		n.dialing()
		wg.Done()
	}()
	wg.Wait()

	n.Wg.Add(Conf.NServers * 2)
	for i := 0; i < Conf.NServers; i++ {
		go n.RecvHandler(i)
		go n.SendHandler(i)
	}
}

func (n *NetTCP) RecvHandler(from int) {
	defer n.Wg.Done()
	readBuf := make([]byte, Conf.IoBufSize)
	for {
		var m Msg
		err, _ := m.ReadUnmarshal(n.Readers[from], readBuf)
		if err != nil {
			// maybe: TCP connection is closed or receives an ill-formed message
			return
		}
		n.RecvChan <- m
	}
}

func (n *NetTCP) SendHandler(to int) {
	defer n.Wg.Done()
	for {
		select {
		case <-n.Done:
			return
		case m := <-n.SendChan[to]:
			WriteFlush(n.Writers[to], m)
		}
	}
}

func (n *NetTCP) PrintStatus() {
	fmt.Println("net layer id =", n.Id)
	for _, c := range n.SendConn {
		fmt.Println("\t ", (*c).LocalAddr(), "\tsend to  \t", (*c).RemoteAddr())
	}
	for _, c := range n.RecvConn {
		fmt.Println("\t ", (*c).LocalAddr(), "\trecv from\t", (*c).RemoteAddr())
	}
	fmt.Println()
}

func (n *NetTCP) Close() {
	for _, c := range n.SendConn {
		_ = (*c).Close()
	}
	for _, c := range n.RecvConn {
		_ = (*c).Close()
	}
	close(n.Done)
	_ = n.Listener.Close()
	n.Wg.Wait()
}
